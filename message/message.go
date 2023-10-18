package message

import "fmt"

type MessageType int

type Message struct {
	Type MessageType
	Data []byte
}

// Same as gorilla Message.
// RFC 6455, section 11.8
const (
	MTText   MessageType = 1
	MTBinary MessageType = 2
	// MessageTypeClose  MessageType = 8
	// MessageTypePing   MessageType = 9
	// MessageTypePong   MessageType = 10
)

type PacketType int

const (
	PTOpen PacketType = iota
	PTClose
	PTPing
	PTPong
	PTMessage
	PTUpgrade
	PTNoop
)

func (pt PacketType) Bytes() []byte {
	return []byte{byte(pt) + '0'}
}

func (pt PacketType) Byte() byte {
	return byte(pt) + '0'
}

func ParsePacketType(b byte) (PacketType, error) {
	pt := PacketType(b - '0')
	if pt < PTOpen || pt > PTNoop {
		return 0, fmt.Errorf("packet type invalid: %c", b)
	}
	return pt, nil
}
