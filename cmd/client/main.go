package main

import (
	"fmt"
	"log"
	"time"

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

	pubChan, err := conn.Channel()
	if err != nil {
		log.Fatalf("Error creating publishing chann, Error: %v\n", err)
	}
	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.ArmyMovesPrefix+"."+userName,
		routing.ArmyMovesPrefix+".*",
		pubsub.Transient,
		handlerMove(gameState, pubChan),
	)
	if err != nil {
		log.Fatalf("Error creating and binding the transient queue %s\nError: %v", routing.ArmyMovesPrefix+"*", err)
	}

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.WarRecognitionsPrefix,
		routing.WarRecognitionsPrefix+".*",
		pubsub.Durable,
		handlerWarFromMove(gameState, pubChan),
	)
	if err != nil {
		log.Fatalf("Error creating and binding the durable queue %s\nError: %v", routing.ArmyMovesPrefix+"*", err)
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

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState, channel *amqp.Channel) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(am gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(am)
		switch outcome {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(
				channel,
				routing.ExchangePerilTopic,
				routing.WarRecognitionsPrefix+"."+gs.GetPlayerSnap().Username,
				gamelogic.RecognitionOfWar{
					Attacker: am.Player,
					Defender: gs.GetPlayerSnap(),
				},
			)
			if err != nil {
				fmt.Println("Error marshaling and publishing to the war prefix queue")
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			return pubsub.NackDiscard
		}
	}
}

func handlerWarFromMove(gs *gamelogic.GameState, channel *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(war gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		outcome, winner, loser := gs.HandleWar(war)
		var message string

		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			message = fmt.Sprintf("%s won a war against %s", winner, loser)
		case gamelogic.WarOutcomeYouWon:
			message = fmt.Sprintf("%s won a war against %s", winner, loser)
		case gamelogic.WarOutcomeDraw:
			message = fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
		default:
			fmt.Println("Battle outcome didn't match any of the registered outcomes")
			return pubsub.NackDiscard
		}

		err := publishGameLog(channel, gs.GetUsername(), message)
		if err != nil {
			return pubsub.NackRequeue
		}
		return pubsub.Ack
	}

}

func publishGameLog(ch *amqp.Channel, username, message string) error {
	gLog := routing.GameLog{
		CurrentTime: time.Now(),
		Message:     message,
		Username:    username,
	}

	err := pubsub.PublishGob(
		ch,
		routing.ExchangePerilTopic,
		routing.GameLogSlug+"."+username,
		gLog,
	)

	if err != nil {
		return err
	}
	return nil
}
