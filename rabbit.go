package main

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

const queueName = "crawler.urls"

// RabbitQueue shares the URL frontier between any number of worker processes.
type RabbitQueue struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	mu   sync.Mutex // serialises publishes from the worker goroutines
}

func NewRabbitQueue(url string, prefetch int) (*RabbitQueue, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	// durable queue so the frontier survives a broker restart
	if _, err := ch.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		conn.Close()
		return nil, err
	}
	// don't let one worker hoard messages the others could be working on
	if err := ch.Qos(prefetch, 0, false); err != nil {
		conn.Close()
		return nil, err
	}
	return &RabbitQueue{conn: conn, ch: ch}, nil
}

func (q *RabbitQueue) Publish(ctx context.Context, t Task) error {
	body, err := json.Marshal(t)
	if err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.ch.PublishWithContext(ctx, "", queueName, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

func (q *RabbitQueue) Consume(ctx context.Context) (<-chan Message, error) {
	// autoAck is off: a message is only removed once the worker acks it,
	// so a crash mid-fetch means the URL gets redelivered.
	deliveries, err := q.ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return nil, err
	}
	out := make(chan Message)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-deliveries:
				if !ok {
					return
				}
				var t Task
				if err := json.Unmarshal(d.Body, &t); err != nil {
					log.Printf("dropping malformed message: %v", err)
					d.Reject(false)
					continue
				}
				select {
				case out <- Message{Task: t, Ack: func() { d.Ack(false) }}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}

func (q *RabbitQueue) Close() error {
	q.ch.Close()
	return q.conn.Close()
}
