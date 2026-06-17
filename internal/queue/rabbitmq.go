package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const defaultQueueName = "creator.generation"

type RabbitMQ struct {
	conn      *amqp.Connection
	pubCh     *amqp.Channel
	queueName string
}

func NewRabbitMQ(url string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	q := &RabbitMQ{conn: conn, pubCh: pubCh, queueName: defaultQueueName}
	if err := q.declare(pubCh); err != nil {
		_ = pubCh.Close()
		_ = conn.Close()
		return nil, err
	}
	return q, nil
}

func (q *RabbitMQ) Publish(msg Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return q.pubCh.PublishWithContext(ctx, "", q.queueName, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

func (q *RabbitMQ) Consume() (<-chan Message, error) {
	ch, err := q.conn.Channel()
	if err != nil {
		return nil, err
	}
	if err := q.declare(ch); err != nil {
		_ = ch.Close()
		return nil, err
	}
	deliveries, err := ch.Consume(q.queueName, "", false, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		return nil, err
	}
	out := make(chan Message)
	go func() {
		defer close(out)
		defer ch.Close()
		for d := range deliveries {
			var msg Message
			if err := json.Unmarshal(d.Body, &msg); err != nil {
				_ = d.Nack(false, false)
				continue
			}
			out <- msg
			_ = d.Ack(false)
		}
	}()
	return out, nil
}

func (q *RabbitMQ) Len() int {
	info, err := q.pubCh.QueueInspect(q.queueName)
	if err != nil {
		return 0
	}
	return info.Messages
}

func (q *RabbitMQ) Close() error {
	var err error
	if q.pubCh != nil {
		err = q.pubCh.Close()
	}
	if q.conn != nil {
		if closeErr := q.conn.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}

func (q *RabbitMQ) declare(ch *amqp.Channel) error {
	_, err := ch.QueueDeclare(q.queueName, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}
	return nil
}
