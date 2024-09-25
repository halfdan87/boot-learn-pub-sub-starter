package main

import (
	"fmt"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
	"strconv"
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

	name, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Println("Error creating channel:", err)
		return
	}

	fmt.Println("Welcome to the Peril client!")
	_, _, err = pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, "pause."+name, routing.PauseKey, pubsub.TransientQueue)
	if err != nil {
		fmt.Println("Error declaring and binding queue:", err)
		return
	}

	gamestate := gamelogic.NewGameState(name)

	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect, "pause."+name, routing.PauseKey, pubsub.TransientQueue, handlerPause(gamestate))
	if err != nil {
		fmt.Println("Error subscribing to queue:", err)
		return
	}

	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}
		if words[0] == "spawn" {
			fmt.Println("Spawning a unit...")
			gamestate.CommandSpawn(words)
			continue
		}
		if words[0] == "move" {
			fmt.Println("Moving units...")
			mv, err := gamestate.CommandMove(words)
			if err != nil {
				fmt.Println(err)
				continue
			}
			pubsub.PublishJSON(ch, routing.ExchangePerilTopic, routing.ArmyMovesPrefix, mv)
			continue
		}
		if words[0] == "status" {
			gamestate.CommandStatus()
			continue
		}
		if words[0] == "quit" {
			gamelogic.PrintQuit()
			break
		}
		if words[0] == "help" {
			gamelogic.PrintClientHelp()
			continue
		}
		if words[0] == "spam" {
			if len(words) < 2 {
				fmt.Println("Usage: spam <n>")
				continue
			}
			n, err := strconv.Atoi(words[1])
			if err != nil {
				fmt.Println("Error: spam must be an integer")
				continue
			}
			for i := 0; i < n; i++ {
				pubsub.PublishJSON(ch, routing.ExchangePerilTopic, routing.GameLogSlug+"."+name, gamelogic.GetMaliciousLog())
			}
			continue
		}

		fmt.Print(fmt.Errorf("Unknown command: %v", words[0]))
	}
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) {
	defer fmt.Print("> ")
	return gs.HandlePause
}
