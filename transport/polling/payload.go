package polling

import (
	"io"
	"net/http"

	"engine.igo/v4/message"
)

type Payload struct {
	writeCh   chan io.Writer
	writeDone chan struct{}

	readCh  chan *packetReader
	readErr chan error

	pauseCh chan struct{}

	closeCh chan struct{}
}

func NewPayload() *Payload {
	return &Payload{
		writeCh:   make(chan io.Writer),
		writeDone: make(chan struct{}),

		readCh:  make(chan *packetReader),
		readErr: make(chan error),

		pauseCh: make(chan struct{}),

		closeCh: make(chan struct{}),
	}
}

func (p *Payload) Pause() {
	close(p.pauseCh)
}

func (p *Payload) Close(pt message.PacketType) {
	select {
	case <-p.closeCh:
		// no-op
	case iow := <-p.writeCh:
		w := packetWriter{
			w:    iow,
			pt:   pt,
			done: p.writeDone,
		}
		defer w.Close()
		w.Write(nil)

	default:
		close(p.closeCh)
	}
}

func (p *Payload) PutWriter(w http.ResponseWriter) error {
	select {
	case <-p.pauseCh:
		w.WriteHeader(http.StatusOK)
		w.Write(message.PTNoop.Bytes())
		return nil
	default:
	}

	select {
	case <-p.pauseCh:
		w.WriteHeader(http.StatusOK)
		w.Write(message.PTNoop.Bytes())
		return nil
	case p.writeCh <- w:
		<-p.writeDone
		return nil
	}
}

func (p *Payload) GetWriter(pt message.PacketType) (io.WriteCloser, error) {
	select {
	case <-p.pauseCh:
		return nil, ErrUpgrade
	default:
	}

	select {
	case <-p.pauseCh:
		return nil, ErrUpgrade
	case <-p.closeCh:
		return nil, ErrClose
	case w := <-p.writeCh:
		return &packetWriter{
			w:    w,
			pt:   pt,
			done: p.writeDone,
		}, nil
	}
}

func (p *Payload) PutReader(r io.Reader) error {
	p.readCh <- &packetReader{
		r:     r,
		errCh: p.readErr,
	}

	err := <-p.readErr
	return err
}

func (p *Payload) GetReader() (message.MessageType, message.PacketType, io.ReadCloser, error) {
	select {
	case <-p.pauseCh:
		return 0, 0, nil, ErrUpgrade
	default:
	}

	select {
	case <-p.pauseCh:
		return 0, 0, nil, ErrUpgrade
	case r := <-p.readCh:
		bs := make([]byte, 1)
		_, err := r.Read(bs)
		if err != nil && err != io.EOF {
			return 0, 0, nil, err
		}
		b := bs[0]

		var (
			mt message.MessageType
			pt message.PacketType
		)

		if b == 'b' {
			mt = message.MTBinary
		} else {
			mt = message.MTText
			pt, err = message.ParsePacketType(b)
			if err != nil {
				r.error(err)
				return 0, 0, nil, err
			}
		}
		return mt, pt, r, nil
	}
}
