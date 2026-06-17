package queue

type Message struct {
	TaskID string `json:"task_id"`
}

type Queue interface {
	Publish(Message) error
	Consume() (<-chan Message, error)
	Len() int
}
