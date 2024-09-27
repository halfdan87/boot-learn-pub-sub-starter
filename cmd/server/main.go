package main

import (
	"fmt"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	gamelogic.PrintServerHelp()

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

	pubsub.SubscribeGob(conn, routing.ExchangePerilTopic, routing.GameLogSlug, routing.GameLogSlug+".*", pubsub.DurableQueue, handlerGameLog())

	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}

		if input[0] == "pause" {
			fmt.Println("Pausing the game...")
			data := routing.PlayingState{IsPaused: true}
			pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, data)
		}
		if input[0] == "resume" {
			fmt.Println("Resuming the game...")
			data := routing.PlayingState{IsPaused: false}
			pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, data)
		}
		if input[0] == "quit" {
			fmt.Println("Quitting the game...")
			break
		}

		gamelogic.PrintServerHelp()
	}
}

func handlerGameLog() func(gamelog routing.GameLog) pubsub.AckType {
	return func(gamelog routing.GameLog) pubsub.AckType {
		defer fmt.Print("> ")
		gamelogic.WriteLog(gamelog)
		return pubsub.Ack
	}
}
