package polling

import (
	"io"
	"net/http"

	"github.com/taogames/engine.igo/message"
)

type Payload struct {
	writeCh   chan io.Writer
	writeDone chan struct{}

	readCh    chan *packetReader
	readErrCh chan error

	pauseCh chan struct{}

	closeCh chan struct{}
}

func NewPayload() *Payload {
	return &Payload{
		writeCh:   make(chan io.Writer),
		writeDone: make(chan struct{}),

		readCh:    make(chan *packetReader),
		readErrCh: make(chan error),

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
		errCh: p.readErrCh,
	}
	err := <-p.readErrCh
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
		return r.parse()
	}
}
