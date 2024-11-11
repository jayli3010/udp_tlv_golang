package tlv

import (
	"encoding/binary"
	"errors"
)

type TLVMessage struct {
	Type   uint8
	Length uint16
	Value  []byte
}

func Encode(msg TLVMessage) ([]byte, error) {
	result := make([]byte, 3+len(msg.Value))
	result[0] = msg.Type
	binary.BigEndian.PutUint16(result[1:3], uint16(len(msg.Value)))
	copy(result[3:], msg.Value)
	return result, nil
}

func Decode(data []byte) (TLVMessage, error) {
	if len(data) < 3 {
		return TLVMessage{}, errors.New("data too short")
	}

	msgType := data[0]
	length := binary.BigEndian.Uint16(data[1:3])

	if uint16(len(data)-3) < length {
		return TLVMessage{}, errors.New("incomplete data")
	}

	return TLVMessage{
		Type:   msgType,
		Length: length,
		Value:  data[3 : 3+length],
	}, nil
}
