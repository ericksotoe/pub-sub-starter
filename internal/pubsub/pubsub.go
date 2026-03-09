package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType int

const (
	Transient SimpleQueueType = 0
	Durable   SimpleQueueType = 1
)

type AckType int

const (
	Ack AckType = iota
	NackRequeue
	NackDiscard
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

func SubscribeJSON[T any](conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType, handler func(T) AckType) error {
	chanl, queue, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}

	chanl.Qos(10, 0, false)

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
			ackType := handler(msg)
			switch ackType {
			case Ack:
				delivery.Ack(false)
				fmt.Println("Message acknowledged (Ack)")
			case NackRequeue:
				delivery.Nack(false, true)
				fmt.Println("Message failed, discarding (NackRequeue)")
			case NackDiscard:
				delivery.Nack(false, false)
				fmt.Println("Message failed, discarding (NackDiscard)")
			}
			fmt.Print("> ")
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

	args := amqp.Table{"x-dead-letter-exchange": "peril_dlx"}
	queue, err := channl.QueueDeclare(queueName, isDurable, autoDelete, isExclusive, false, args)
	if err != nil {
		return &amqp.Channel{}, amqp.Queue{}, err
	}

	err = channl.QueueBind(queueName, key, exchange, false, nil)
	if err != nil {
		return &amqp.Channel{}, amqp.Queue{}, err
	}
	return channl, queue, nil
}

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(val)
	if err != nil {
		return err
	}

	toPublish := amqp.Publishing{
		ContentType: "application/gob",
		Body:        buffer.Bytes(),
	}
	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, toPublish)
	if err != nil {
		return err
	}
	return nil
}

func SubscribeGob[T any](conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType, handler func(T) AckType) error {
	chanl, queue, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}

	chanl.Qos(10, 0, false)

	deliveries, err := chanl.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for delivery := range deliveries {
			if delivery.ContentType != "application/gob" {
				fmt.Println("Content type is not of type gob")
				continue
			}
			var msg T
			reader := bytes.NewReader(delivery.Body)
			dec := gob.NewDecoder(reader)
			err := dec.Decode(&msg)
			if err != nil {
				fmt.Printf("Error unmarshalling using Gob: %s\n", err)
				continue
			}
			ackType := handler(msg)
			switch ackType {
			case Ack:
				delivery.Ack(false)
				fmt.Println("Message acknowledged (Ack)")
			case NackRequeue:
				delivery.Nack(false, true)
				fmt.Println("Message failed, requeueing (NackRequeue)")
			case NackDiscard:
				delivery.Nack(false, false)
				fmt.Println("Message failed, discarding (NackDiscard)")
			}
			// fmt.Print("> ")
		}
	}()

	return nil
}
