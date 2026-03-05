package main

import (
	"fmt"
	"log"

	"github.com/ericksotoe/pub-sub-starter/internal/gamelogic"
	"github.com/ericksotoe/pub-sub-starter/internal/pubsub"
	"github.com/ericksotoe/pub-sub-starter/internal/routing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril client...")

	const connectionString = "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(connectionString)
	if err != nil {
		log.Fatalf("Error creating the connection at: %s\nError: %v", connectionString, err)
	}
	defer conn.Close()
	fmt.Println("Connection successful")

	userName, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalf("Error when logging in the user\nError: %v", err)
	}

	gameState := gamelogic.NewGameState(userName)

	queueName := fmt.Sprintf("%s.%s", routing.PauseKey, userName)
	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilDirect,
		queueName,
		routing.PauseKey,
		pubsub.Transient,
		handlerPause(gameState),
	)
	if err != nil {
		log.Fatalf("Error creating and binding the transient queue %s\nError: %v", queueName, err)
	}

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.ArmyMovesPrefix+"."+userName,
		routing.ArmyMovesPrefix+".*",
		pubsub.Transient,
		handlerMove(gameState),
	)
	if err != nil {
		log.Fatalf("Error creating and binding the transient queue %s\nError: %v", routing.ArmyMovesPrefix+"*", err)
	}

	chnl, err := conn.Channel()
	if err != nil {
		fmt.Printf("Error setting up channel connection, Error: %v\n", err)
	}
	defer chnl.Close()
	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}

		switch words[0] {
		case "spawn":

			err := gameState.CommandSpawn(words)
			if err != nil {
				fmt.Println(err)
				continue
			}
		case "move":
			armyMove, err := gameState.CommandMove(words)
			if err != nil {
				fmt.Println(err)
				continue
			}
			err = pubsub.PublishJSON(chnl, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+userName, armyMove)
			if err != nil {
				fmt.Printf("Error publishing move to exchange %s, Error: %v\n", routing.ExchangePerilTopic, err)
				continue
			}
			fmt.Println("Move was published successfully")
		case "status":
			gameState.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			return
		default:
			fmt.Println("Please input a compatible command type help to list them")
			continue
		}
	}

}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) {
	return func(ps routing.PlayingState) {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
	}
}

func handlerMove(gs *gamelogic.GameState) func(gamelogic.ArmyMove) {
	return func(am gamelogic.ArmyMove) {
		defer fmt.Print("> ")
		gs.HandleMove(am)
	}
}
