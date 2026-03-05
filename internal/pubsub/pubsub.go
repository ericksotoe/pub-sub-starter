package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType int

const (
	Transient SimpleQueueType = 0
	Durable   SimpleQueueType = 1
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	jsonBytes, err := json.Marshal(val)
	if err != nil {
		return err
	}

	toPublish := amqp.Publishing{
		ContentType: "application/json",
		Body:        jsonBytes,
	}
	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, toPublish)
	if err != nil {
		return err
	}
	return nil
}

func SubscribeJSON[T any](conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType, handler func(T)) error {
	chanl, queue, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}

	deliveries, err := chanl.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for delivery := range deliveries {
			var msg T
			err := json.Unmarshal(delivery.Body, &msg)
			if err != nil {
				fmt.Printf("Error unmarshalling: %s\n", err)
				continue
			}
			handler(msg)

			delivery.Ack(false)
		}
	}()

	return nil
}

func DeclareAndBind(conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType) (*amqp.Channel, amqp.Queue, error) {
	channl, err := conn.Channel()
	if err != nil {
		return &amqp.Channel{}, amqp.Queue{}, err
	}

	var isDurable, autoDelete, isExclusive bool
	switch queueType {
	case Transient:
		isDurable = false
		autoDelete = true
		isExclusive = true
	case Durable:
		isDurable = true
		autoDelete = false
		isExclusive = false
	}

	queue, err := channl.QueueDeclare(queueName, isDurable, autoDelete, isExclusive, false, nil)
	if err != nil {
		return &amqp.Channel{}, amqp.Queue{}, err
	}

	err = channl.QueueBind(queueName, key, exchange, false, nil)
	if err != nil {
		return &amqp.Channel{}, amqp.Queue{}, err
	}
	return channl, queue, nil
}
