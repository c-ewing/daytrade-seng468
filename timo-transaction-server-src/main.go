package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// CONSTANTS:
const MAX_CONNECTION_RETRIES = 5
const TIME_BETWEEN_RETRIES_SECONDS = 5
const MONGODB_TIMEOUT_SECONDS = 5
const RABBITMQ_TIMEOUT_SECONDS = 5

var HOSTNAME = Environment_variable_or_default("DOCKER_HOSTNAME", "TriggerListener")
var RABBITMQ_CONNECTION_STRING = Environment_variable_or_default("RABBITMQ_CONNECTION_STRING", "amqp://guest:guest@rabbitmq:5672/")
var MONGODB_CONNECTION_STRING = Environment_variable_or_default("MONGODB_CONNECTION_STRING", "mongodb://root:example@mongodb:27017/?retryWrites=true&w=majority")

var MONGO_CLIENT = Connect_mongodb()

// FUNCTIONS:
func main() {
	// This container should be started after RabbitMQ using a health check

	// First connect to RabbitMQ
	rabbit_connection := Connect_rabbitmq()
	defer rabbit_connection.Close()
	// Open a channel to communicate over
	rabbitmq_channel := Open_rabbitmq_channel(rabbit_connection)
	defer rabbitmq_channel.Close()

	// Declare a queue to receive commands from
	command_queue, err := rabbitmq_channel.QueueDeclare(
		"command_queue", // name
		true,            // durable
		false,           // delete when unused
		false,           // exclusive
		false,           // no-wait
		nil,             // arguments
	)

	if err != nil {
		Log_error_event(CommandMessage{}, fmt.Sprintf("Failed to declare a queue: %s", err))
		panic(err)
	}

	// Set Quality of Service to limit this worker to grabbing one command at a time
	err = rabbitmq_channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)

	if err != nil {
		Log_error_event(CommandMessage{}, fmt.Sprintf("Failed to set QoS: %s", err))
		panic(err)
	}

	// Set up the message consumer
	msgs, err := rabbitmq_channel.Consume(
		command_queue.Name, // queue
		"",                 // consumer
		false,              // auto-ack
		false,              // exclusive
		false,              // no-local
		false,              // no-wait
		nil,                // args
	)

	if err != nil {
		Log_error_event(CommandMessage{}, fmt.Sprintf("Failed to register a consumer: %s", err))
		panic(err)
	}

	// Create a channel to block forever
	var forever chan struct{}

	// Start a goroutine to handle messages
	go process_messages(msgs, rabbitmq_channel)

	log.Printf(" [*] Waiting for commands. To exit press CTRL+C")
	Log_debug_event(CommandMessage{}, "Started Transaction Server")
	// Block forever
	<-forever
}

// Helper Functions
func process_messages(msgs <-chan amqp.Delivery, rabbitmq_channel *amqp.Channel) {
	for message := range msgs {
		// Unmarshal the message to a CommandMessage struct
		var command_message CommandMessage
		err := json.Unmarshal(message.Body, &command_message)

		if err != nil {
			Log_error_event(CommandMessage{}, fmt.Sprintf("Error unmarshalling message: %s", err))
			message.Ack(false)
			continue
		}

		Log_user_command(command_message)

		response := []byte{}

		switch command_message.Command {
		case "ADD":
			response = []byte(Command_add(command_message))
			message.Ack(false)
		case "QUOTE":
			response = []byte(Command_quote(command_message, rabbitmq_channel))
			message.Ack(false)
		case "BUY":
			response = []byte(Command_buy(command_message, rabbitmq_channel))
			message.Ack(false)
		case "COMMIT_BUY":
			response = []byte(Command_commit_buy(command_message))
			message.Ack(false)
		case "CANCEL_BUY":
			response = []byte(Command_cancel_buy(command_message))
			message.Ack(false)
		case "SELL":
			response = []byte(Command_sell(command_message, rabbitmq_channel))
			message.Ack(false)
		case "COMMIT_SELL":
			response = []byte(Command_commit_sell(command_message))
			message.Ack(false)
		case "CANCEL_SELL":
			response = []byte(Command_cancel_sell(command_message))
			message.Ack(false)
		case "SET_BUY_AMOUNT":
			response = []byte(Command_set_buy_amount(command_message))
			message.Ack(false)
		case "SET_BUY_TRIGGER":
			response = []byte(Command_set_buy_trigger(command_message))
			message.Ack(false)
		case "CANCEL_SET_BUY":
			response = []byte(Command_cancel_set_buy(command_message))
			message.Ack(false)
		case "SET_SELL_AMOUNT":
			response = []byte(Command_set_sell_amount(command_message))
			message.Ack(false)
		case "SET_SELL_TRIGGER":
			response = []byte(Command_set_sell_trigger(command_message))
			message.Ack(false)
		case "CANCEL_SET_SELL":
			response = []byte(Command_cancel_set_sell(command_message))
			message.Ack(false)
		case "DUMPLOG":
			response = []byte(Command_dumplog(command_message))
			message.Ack(false)
		case "DISPLAY_SUMMARY":
			response = []byte(Command_display_summary(command_message))
			message.Ack(false)

		default:
			Log_error_event(command_message, fmt.Sprintf("Unknown command: %s in message: %s", command_message.Command, message.Body))
		}

		// If there is a response, send it back to the client
		response_timeout, cancel := context.WithTimeout(context.Background(), RABBITMQ_TIMEOUT_SECONDS*time.Second)

		if len(response) != 0 {
			// Publish the response to the response queue
			err := rabbitmq_channel.PublishWithContext(
				response_timeout, // context
				"",               // exchange
				message.ReplyTo,  // routing key
				false,            // mandatory
				false,            // immediate
				amqp.Publishing{
					ContentType:   "text/plain",
					CorrelationId: message.CorrelationId,
					Body:          response,
				})

			if err != nil {
				Log_error_event(command_message, fmt.Sprintf("Failed to publish a response: %s", err))
				panic(err)
			}
		}

		cancel()
	}
}
