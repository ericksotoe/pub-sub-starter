package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

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
		log.Fatalf("Error opening the channel, error: %v\n", err)
	}

	val := routing.PlayingState{IsPaused: true}
	err = pubsub.PublishJSON(chanl, routing.ExchangePerilDirect, routing.PauseKey, val)
	if err != nil {
		log.Fatalf("Error sending the data to the exchange, Error: %v\n", err)
	}

	// wait for ctrl+c
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
	fmt.Println("RabbitMQ connection closed")
}
