package main

import (
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	connectionString := "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(connectionString)
	if err != nil {
		fmt.Println("Error connecting to RabbitMQ:", err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		fmt.Println("Error creating channel:", err)
		return
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"peril_topic", // name
		false,         // durable
		false,         // delete when unused
		false,         // exclusive
		false,         // no-wait
		nil,           // arguments
	)
	if err != nil {
		fmt.Println("Error declaring queue:", err)
		return
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
		fmt.Println("Error consuming:", err)
		return
	}

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			fmt.Printf("Received a message: %s\n", d.Body)
		}
	}()

	fmt.Println("Waiting for messages. To exit press CTRL+C")
	<-forever
}
