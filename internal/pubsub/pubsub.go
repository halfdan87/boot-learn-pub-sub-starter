package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishJSON[T any](ch *amqp.Channel, exchange, routingKey string, msg T) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("error marshalling message: %v", err)
	}

	err = ch.PublishWithContext(
		context.Background(),
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})

	if err != nil {
		return fmt.Errorf("error publishing message: %v", err)
	}
	return nil
}

func DeclareAndBind(conn *amqp.Connection, exchange, queueName, bindingKey string, simpleQueueType QueueType) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("error creating channel: %v", err)
	}

	q, err := ch.QueueDeclare(
		queueName,                         // name
		simpleQueueType == DurableQueue,   // durable
		simpleQueueType == TransientQueue, // delete when unused
		simpleQueueType == TransientQueue, // exclusive
		false,                             // no-wait
		nil,                               // arguments
	)
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("error declaring queue: %v", err)
	}

	err = ch.QueueBind(
		q.Name,     // queue name
		bindingKey, // routing key
		exchange,   // exchange
		false,
		nil,
	)
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("error binding queue: %v", err)
	}

	return ch, q, nil
}

type QueueType int

const (
	TransientQueue QueueType = iota
	DurableQueue
)

func SubscribeJSON[T any](conn *amqp.Connection, exchange, queueName, bindingKey string, simpleQueueType QueueType, handler func(T)) error {
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("error creating channel: %v", err)
	}

	_, q, err := DeclareAndBind(conn, exchange, queueName, bindingKey, simpleQueueType)
	if err != nil {
		return fmt.Errorf("error declaring and binding queue: %v", err)
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return fmt.Errorf("error consuming: %v", err)
	}

	go func() {
		for d := range msgs {
			var msg T
			err := json.Unmarshal(d.Body, &msg)
			if err != nil {
				fmt.Printf("Error unmarshalling message: %v\n", err)
				continue
			}
			handler(msg)
			d.Ack(false)
		}
	}()

	return nil
}
