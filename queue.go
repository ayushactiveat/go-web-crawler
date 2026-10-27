package main

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type QueueManager struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	q    amqp.Queue
}

func NewQueueManager(url string) (*QueueManager, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	q, err := ch.QueueDeclare("url_queue", false, false, false, false, nil)
	if err != nil {
		return nil, err
	}
	return &QueueManager{conn: conn, ch: ch, q: q}, nil
}

func (qm *QueueManager) Publish(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return qm.ch.PublishWithContext(ctx, "", qm.q.Name, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte(url),
	})
}

func (qm *QueueManager) Consume() (<-chan amqp.Delivery, error) {
	return qm.ch.Consume(qm.q.Name, "", true, false, false, false, nil)
}

func (qm *QueueManager) Close() {
	qm.ch.Close()
	qm.conn.Close()
}
