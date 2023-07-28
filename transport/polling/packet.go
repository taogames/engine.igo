package polling

import (
	"io"

	"engine.igo/v4/message"
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

func (r *packetReader) error(err error) {
	r.errCh <- err
}
