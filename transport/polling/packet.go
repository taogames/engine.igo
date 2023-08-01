package polling

import (
	"io"

	"github.com/taogames/engine.igo/message"
)

type packetWriter struct {
	w    io.Writer
	pt   message.PacketType
	done chan<- struct{}
}

func (w *packetWriter) Write(bs []byte) (int, error) {
	n1, err := w.w.Write(w.pt.Bytes())
	if err != nil {
		return n1, err
	}

	n2, err := w.w.Write(bs)
	return n1 + n2, err
}

func (w *packetWriter) Close() error {
	w.done <- struct{}{}
	return nil
}

type packetReader struct {
	r     io.Reader
	errCh chan<- error
}

func (r *packetReader) Read(bs []byte) (int, error) {
	return r.r.Read(bs)
}

func (r *packetReader) Close() error {
	r.errCh <- nil
	return nil
}

func (r *packetReader) parse() (message.MessageType, message.PacketType, io.ReadCloser, error) {
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
			r.errCh <- err
			return 0, 0, nil, err
		}
	}
	return mt, pt, r, nil
}
