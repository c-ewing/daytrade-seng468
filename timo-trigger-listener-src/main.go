package main

import (
	"fmt"
	"log"
)

var HOSTNAME = Environment_variable_or_default("DOCKER_HOSTNAME", "TriggerListener")
var RABBITMQ_CONNECTION_STRING = Environment_variable_or_default("RABBITMQ_CONNECTION_STRING", "amqp://guest:guest@rabbitmq:5672/")
var MONGODB_CONNECTION_STRING = Environment_variable_or_default("MONGODB_CONNECTION_STRING", "mongodb://root:example@mongodb:27017/?retryWrites=true&w=majority")

// Connect to mongoDB for logging and accessing user accounts
var MONGO_CLIENT = Connect_mongodb()

const MONGODB_TIMEOUT_SECONDS = 5
const RABBITMQ_TIMEOUT_SECONDS = 5
const MAX_CONNECTION_RETRIES = 5
const TIME_BETWEEN_RETRIES_SECONDS = 5

func main() {
	// Subscribe to stock updates for certain symbols
	// When a stock update is received, get the list of users with triggers on that symbol from trigger-symbols redis cache
	// Query mongo for those users and their associated trigger prices
	// If one of the trigger prices is met, perform the associated action if it is still valid (enough stock/money in the account)
	// Log the action and update the user's account

	// Connect to RabbitMQ
	rabbit_connection := Connect_rabbitmq()
	defer rabbit_connection.Close()

	// Open a channel to communicate over
	rabbitmq_channel := Open_rabbitmq_channel(rabbit_connection)
	defer rabbitmq_channel.Close()

	// Declare a queue to receive updates on:
	// We start with no subscriptions, those are added with QueueBind later
	stock_update_queue, err := rabbitmq_channel.QueueDeclare(
		"",    // name, randomized as it is unimportant
		false, // durable - we don't want to save these messages if we crash
		false, // delete when unused
		true,  // exclusive - we don't want other listeners to be able to access this queue
		false, // no-wait
		nil,   // arguments
	)

	if err != nil {
		Log_error_event(CommandMessage{}, fmt.Sprintf("Failed to declare a queue: %s", err))
		panic(err)
	}

	// Create the exchange for stock updates if it doesn't exist:
	err = rabbitmq_channel.ExchangeDeclare(
		"stock_price_updates", // name of the exchange
		"direct",              // type of exchange
		true,                  // durable
		false,                 // delete when complete
		false,                 // internal
		false,                 // no-wait
		nil,                   // arguments
	)

	if err != nil {
		Log_error_event(CommandMessage{}, fmt.Sprintf("Failed to declare an exchange: %s", err))
		panic(err)
	}

	// Setup a consumer to receive messages from the queue
	msgs, err := rabbitmq_channel.Consume(
		stock_update_queue.Name, // queue
		"",                      // consumer
		true,                    // auto-ack
		false,                   // exclusive
		false,                   // no-local
		false,                   // no-wait
		nil,                     // args
	)

	if err != nil {
		Log_error_event(CommandMessage{}, fmt.Sprintf("Failed to register a consumer: %s", err))
		panic(err)
	}

	// Just to get ti to compile for now:

	for message := range msgs {
		log.Printf("Received a message: %s", message.Body)
	}

	// Log Startup:
	Log_debug_event(CommandMessage{}, "Started Trigger Listener")
}
