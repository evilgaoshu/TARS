package contracts

import "encoding/json"

func EncodeChannelMessage(msg ChannelMessage) ([]byte, error) {
	return json.Marshal(msg)
}

func DecodeChannelMessage(payload []byte) (ChannelMessage, error) {
	if len(payload) == 0 {
		return ChannelMessage{}, nil
	}
	var msg ChannelMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return ChannelMessage{}, err
	}
	return msg, nil
}
