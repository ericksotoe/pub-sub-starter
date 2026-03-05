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
	fmt.Println("Starting Peril server...")
	const connectionString = "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(connectionString)
	if err != nil {
		log.Fatalf("Error creating the connection at: %s\nError: %v", connectionString, err)
	}
	defer conn.Close()
	fmt.Println("Connection successful")

	chanl, err := conn.Channel()
	if err != nil {
		log.Fatalf("Error creating channel: %v", err)

	}
	_, queue, err := pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilTopic,
		routing.GameLogSlug,
		routing.GameLogSlug+".*",
		pubsub.Durable,
	)
	if err != nil {
		log.Fatalf("Could not subscribe to %s exchange to handle game logs, Error: %v", routing.ExchangePerilTopic, err)
	}
	fmt.Printf("Queue %v declared and bound\n", queue.Name)

	gamelogic.PrintServerHelp()

	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}
		switch words[0] {
		case "pause":
			fmt.Println("Sending pause message")
			val := routing.PlayingState{IsPaused: true}
			err = pubsub.PublishJSON(chanl, routing.ExchangePerilDirect, routing.PauseKey, val)
			if err != nil {
				log.Fatalf("Error sending the data to the exchange, Error: %v\n", err)
			}
		case "resume":
			fmt.Println("Sending resume message")
			val := routing.PlayingState{IsPaused: false}
			err = pubsub.PublishJSON(chanl, routing.ExchangePerilDirect, routing.PauseKey, val)
			if err != nil {
				log.Fatalf("Error sending the data to the exchange, Error: %v\n", err)
			}
		case "quit":
			fmt.Println("Exiting")
			return
		default:
			fmt.Println("Input was not pause, resume, or quit")
		}
	}
}
