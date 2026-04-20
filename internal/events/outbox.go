package events

type OutboxEvent struct {
	ID      string
	Topic   string
	Status  string
	Payload []byte
}
