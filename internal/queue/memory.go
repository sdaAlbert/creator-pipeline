package queue

type MemoryQueue struct {
	ch chan Message
}

func NewMemoryQueue() *MemoryQueue {
	return &MemoryQueue{ch: make(chan Message, 1024)}
}

func (q *MemoryQueue) Publish(msg Message) error {
	q.ch <- msg
	return nil
}

func (q *MemoryQueue) Consume() (<-chan Message, error) {
	return q.ch, nil
}

func (q *MemoryQueue) Len() int {
	return len(q.ch)
}
