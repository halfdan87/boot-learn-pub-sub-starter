package main

import (
	"fmt"
	"strconv"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
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

	_, _, err = pubsub.DeclareAndBind(conn, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+name, routing.ArmyMovesPrefix+".*", pubsub.TransientQueue)
	if err != nil {
		fmt.Println("Error declaring and binding queue:", err)
		return
	}

	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+name, "*", pubsub.TransientQueue, handlerMove(gamestate, ch))
	if err != nil {
		fmt.Println("Error subscribing to queue:", err)
		return
	}

	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix, "*", pubsub.DurableQueue, handlerRecognition(gamestate))
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

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState, ch *amqp.Channel) func(move gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		result := gs.HandleMove(move)
		switch result {
		case gamelogic.MoveOutComeSafe:
			fmt.Println("You are safe from the attack!")
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			fmt.Println("You have been attacked! You are at war with the attacker!")
			// TODO: publish war recognition
			recognition := gamelogic.RecognitionOfWar{
				Attacker: gs.Player,
				Defender: move.Player,
			}
			fmt.Printf("Publishing war recognition: %v\n", recognition)
			err := pubsub.PublishJSON(ch, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix+"."+gs.Player.Username, recognition)
			if err != nil {
				fmt.Println("Error publishing war recognition:", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			fmt.Println("You are already at war with the attacker!")
			return pubsub.NackDiscard
		}
		return pubsub.NackDiscard
	}
}

func handlerRecognition(gs *gamelogic.GameState) func(recognition gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(recognition gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		outcome, winner, loser := gs.HandleWar(recognition)
		switch outcome {
		case gamelogic.WarOutcomeYouWon:
			fmt.Printf("%s has won the war!\n", winner)
			return pubsub.Ack
		case gamelogic.WarOutcomeOpponentWon:
			fmt.Printf("%s has won the war!\n", loser)
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			fmt.Println("The war ended in a draw!")
			return pubsub.Ack
		case gamelogic.WarOutcomeNotInvolved:
			fmt.Println("Not involved in this war.")
			return pubsub.Ack
		case gamelogic.WarOutcomeNoUnits:
			fmt.Println("No units in the same location. No war will be fought.")
			return pubsub.NackDiscard
		}
		fmt.Printf("Unknown war outcome: %v\n", outcome)
		return pubsub.NackDiscard
	}
}
