package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CONSTANTS:
const MAX_CONNECTION_RETRIES = 5
const TIME_BETWEEN_RETRIES_SECONDS = 5

var RABBITMQ_CONNECTION_STRING = environment_variable_or_default("RABBITMQ_CONNECTION_STRING", "amqp://guest:guest@rabbitmq:5672/")

const RABBITMQ_TIMEOUT_SECONDS = 5

// TODO: Find out from Esteban what this actually would be
var MONGODB_CONNECTION_STRING = environment_variable_or_default("MONGODB_CONNECTION_STRING", "mongodb://root:example@mongodb:27017/?retryWrites=true&w=majority")

const MONGODB_TIMEOUT_SECONDS = 5

// FUNCTIONS:
func main() {
	// This container should be started after RabbitMQ using a health check

	// First connect to RabbitMQ
	rabbit_connection := connect_rabbitmq()
	defer rabbit_connection.Close()
	// Open a channel to communicate over
	rabbitmq_channel := open_channel(rabbit_connection)
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
		log.Panicf("Failed to declare a queue: %s", err)
	}

	// Set Quality of Service to limit this worker to grabbing one command at a time
	err = rabbitmq_channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)

	if err != nil {
		log.Panicf("Failed to set QoS: %s", err)
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

	// Connect to MongoDB
	// Use the SetServerAPIOptions() method to set the Stable API version to 1
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(MONGODB_CONNECTION_STRING).SetServerAPIOptions(serverAPI)

	// Create a new mongo_client and connect to the server
	mongo_client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = mongo_client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	// Create a channel to block forever
	var forever chan struct{}

	// Start a goroutine to handle messages
	go process_messages(msgs, mongo_client, rabbitmq_channel)

	log.Printf(" [*] Waiting for commands. To exit press CTRL+C")
	// Block forever
	<-forever
}

// Helper Functions
func process_messages(msgs <-chan amqp.Delivery, mongo_client *mongo.Client, rabbitmq_channel *amqp.Channel) {
	for message := range msgs {
		// Unmarshal the message to a CommandMessage struct
		var command_message CommandMessage
		err := json.Unmarshal(message.Body, &command_message)

		if err != nil {
			log.Printf("Error unmarshalling message: %s", err)
			message.Ack(false)
			continue
		}

		response := []byte{}

		switch command_message.Command {
		case "ADD":
			response = []byte(Command_add(command_message, mongo_client))
			message.Ack(false)
		case "QUOTE":
			response = []byte(Command_quote(command_message, mongo_client, rabbitmq_channel))
			message.Ack(false)
		case "BUY":
			response = []byte(Command_buy(command_message, mongo_client, rabbitmq_channel))
			message.Ack(false)
		case "COMMIT_BUY":
			response = []byte(Command_commit_buy(command_message, mongo_client))
			message.Ack(false)
		case "CANCEL_BUY":
			response = []byte(Command_cancel_buy(command_message, mongo_client))
			message.Ack(false)
		case "SELL":
			response = []byte(Command_sell(command_message, mongo_client, rabbitmq_channel))
			message.Ack(false)
		case "COMMIT_SELL":
			response = []byte(Command_commit_sell(command_message, mongo_client))
			message.Ack(false)
		case "CANCEL_SELL":
			response = []byte(Command_cancel_sell(command_message, mongo_client))
			message.Ack(false)
		// case "SET_BUY_AMOUNT":
		// Command_set_buy_amount(command_parts)
		// case "SET_BUY_TRIGGER":
		// Command_set_buy_trigger(command_parts)
		// case "CANCEL_SET_BUY":
		// Command_cancel_set_buy(command_parts)
		// case "SET_SELL_AMOUNT":
		// Command_set_sell_amount(command_parts)
		// case "SET_SELL_TRIGGER":
		// Command_set_sell_trigger(command_parts)
		// case "CANCEL_SET_SELL":
		// Command_cancel_set_sell(command_parts)
		case "DUMPLOG":
			response = []byte(Command_dumplog(command_message, mongo_client))
			message.Ack(false)
		case "DISPLAY_SUMMARY":
			response = []byte(Command_display_summary(command_message, mongo_client))
			message.Ack(false)

		default:
			log.Printf("Unknown command: %s in message: %s", command_message.Command, message.Body)
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
				log.Printf("Failed to publish a response: %s", err)
			}
		}

		cancel()
	}
}

// HELPERS:
func environment_variable_or_default(key string, def string) string {
	value, exists := os.LookupEnv(key)
	if !exists || value == "" {
		log.Printf(" [warn] Environment variable %s does not exist, using default value: %s", key, def)
		return def
	}
	return value
}

// RETRY FUNCTIONS:
// Connect to RabbitMQ and retry if it fails
func connect_rabbitmq() *amqp.Connection {
	conn, err := amqp.Dial(RABBITMQ_CONNECTION_STRING)

	for i := 0; i < MAX_CONNECTION_RETRIES && err != nil; i++ {
		log.Printf("Failed to connect to '%s', retrying in %d seconds", RABBITMQ_CONNECTION_STRING, TIME_BETWEEN_RETRIES_SECONDS)
		time.Sleep(TIME_BETWEEN_RETRIES_SECONDS * time.Second)
		conn, err = amqp.Dial(RABBITMQ_CONNECTION_STRING)
	}

	if err != nil {
		log.Panicf("Failed to connect to '%s' after %d retries", RABBITMQ_CONNECTION_STRING, MAX_CONNECTION_RETRIES)
	}

	return conn
}

// Open a channel to communicate over and retry if it fails
func open_channel(rabbit_connection *amqp.Connection) *amqp.Channel {
	channel, err := rabbit_connection.Channel()

	for i := 0; i < MAX_CONNECTION_RETRIES && err != nil; i++ {
		log.Printf("Failed to open a channel, retrying in %d seconds", TIME_BETWEEN_RETRIES_SECONDS)
		time.Sleep(TIME_BETWEEN_RETRIES_SECONDS * time.Second)
		channel, err = rabbit_connection.Channel()
	}

	if err != nil {
		log.Panicf("Failed to connect to '%s' after %d retries", RABBITMQ_CONNECTION_STRING, MAX_CONNECTION_RETRIES)
	}

	return channel
}
