// TODO: Logging
// TODO: Reduce the number of panic calls
package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// CONSTANTS:
const MAX_CONNECTION_RETRIES = 5
const TIME_BETWEEN_RETRIES_SECONDS = 5
const REDIS_TIMEOUT_SECONDS = 3
const REDIS_EXPIRY_SECONDS = 10
const RABBITMQ_TIMEOUT_SECONDS = 5

var HOSTNAME = Environment_variable_or_default("DOCKER_HOSTNAME", "TriggerDriver")
var RABBITMQ_CONNECTION_STRING = Environment_variable_or_default("RABBITMQ_CONNECTION_STRING", "amqp://guest:guest@rabbitmq:5672/")
var REDIS_CONNECTION_ADDRESS = Environment_variable_or_default("REDIS_CONNECTION_ADDRESS", "trigger-symbol-redis:6379")
var MONGODB_CONNECTION_STRING = Environment_variable_or_default("MONGODB_CONNECTION_STRING", "mongodb://root:example@mongodb:27017/?retryWrites=true&w=majority")

var MONGO_CLIENT = Connect_mongodb()

// FUNCTIONS:
func main() {
	// This container should be started after the Redis and RabbitMQ containers using a health check
	// However just in case we retry connecting to Redis and RabbitMQ a few times before giving up

	// First connect to Redis. If we connect to RabbitMQ before Redis we may receive requests before the driver is ready to handle them
	// TODO: Use some sort of password storage solution
	redis_client := redis.NewClient(&redis.Options{
		Addr:     REDIS_CONNECTION_ADDRESS,                                       // Connection Address
		Password: "timo-daytrade-redispass-8166d5d6-d622-4b62-bfdd-8c7d1a154c2f", // Password
		DB:       0,                                                              // use default DB
	})

	// Connect to RabbitMQ
	rabbit_connection := Connect_rabbitmq()
	defer rabbit_connection.Close()
	// Open a channel to communicate over
	rabbitmq_channel := Open_rabbitmq_channel(rabbit_connection)
	defer rabbitmq_channel.Close()

	// Create a queue if it doesn't exist already to wait for quote price requests
	// TODO: Add a retry loop heres
	rabbit_queue, err := rabbitmq_channel.QueueDeclare(
		"trigger_symbol", // name
		true,             // durable
		false,            // delete when unused
		false,            // exclusive
		false,            // no-wait, don't wait for a response from the server
		nil,              // arguments
	)

	if err != nil {
		Log_error_event(CommandMessage{}, "Failed to declare a queue")
		panic(err)
		// log.Panicf("Failed to declare a queue: %s", err)
	}

	// Use Quality of Service on the channel to limit the number of messages that can be in flight at once
	// TODO: Add a retry loop here
	err = rabbitmq_channel.Qos(
		1,     // prefetch count (0 = no limit)
		0,     // prefetch size (0 = no limit)
		false, // global (true = apply to entire channel, false = apply to each consumer)
	)

	if err != nil {
		Log_error_event(CommandMessage{}, "Failed to set QoS")
		panic(err)
		// log.Panicf("Failed to set QoS: %s", err)
	}

	// Receive quote price requests through RabbitMQ by registering as a consumer
	// TODO: Add a retry loop here
	rabbit_messages, err := rabbitmq_channel.Consume(
		rabbit_queue.Name, // queue name
		"",                // consumer (empty string = auto generated)
		false,             // auto-ack (false = manual ack), This lets us control when the message is removed from the queue
		false,             // exclusive (true = only this consumer can access the queue)
		false,             // no-local (true = don't send messages to this consumer if they were published from this consumer)
		false,             // no-wait (true = don't wait for a response from the server)
		nil,               // arguments
	)

	if err != nil {
		Log_error_event(CommandMessage{}, "Failed to register as consumer")
		panic(err)
		// log.Panicf("Failed to register as consumer: %s", err)
	}

	var forever chan struct{}

	// Start a goroutine to handle each message
	go func() {
		// Create a Context that creates a timeout for sending the quote price back to the client
		timeout_context, cancel := context.WithTimeout(context.Background(), RABBITMQ_TIMEOUT_SECONDS*time.Second)
		defer cancel()

		for message := range rabbit_messages {
			// Unmarshal the command message from the message body
			var command_message CommandMessage
			err := json.Unmarshal(message.Body, &command_message)

			if err != nil {
				Log_error_event(command_message, "Failed to unmarshal message body")
				panic(err)
				// log.Panicf(" [error] Failed to unmarshal message body: %s", err)
			}

			// log.Printf(" [info] Received trigger %s request from: %s for %s", command_message.Command, command_message.Userid, command_message.StockSymbol)
			Log_debug_event(command_message, "Received trigger request")

			if command_message.Command == "TRIGGER_ADD" {
				// Add this user to the set of users subscribed to this symbol
				_, err := redis_client.SAdd(timeout_context, command_message.StockSymbol, command_message.Userid).Result()

				if err != nil {
					Log_error_event(command_message, "Failed to add user to Redis set")
					panic(err)
					// log.Panicf(" [error] Failed to add %s to Redis set %s with error: %s", command_message.Userid, command_message.StockSymbol, err)
				}
			} else if command_message.Command == "TRIGGER_REMOVE" {
				// Remove this user from the set of users subscribed to this symbol
				_, err := redis_client.SRem(timeout_context, command_message.StockSymbol, command_message.Userid).Result()

				if err != nil {
					Log_error_event(command_message, "Failed to remove user from Redis set")
					panic(err)
					// log.Panicf(" [error] Failed to remove %s from Redis set %s with error: %s", command_message.Userid, command_message.StockSymbol, err)
				}
			} else {
				Log_error_event(command_message, "Received invalid command")
				message.Reject(false) // Reject the message and don't requeue it
				continue
				// log.Printf(" [warn] Received invalid command: %s in message: %+v", command_message.Command, command_message)
			}

			// Acknowledge the message to remove it from the queue now that we have updated the database
			message.Ack(false) // False = Only acknowledge the current message
		}
	}()

	log.Printf(" [info] Waiting for Trigger Subscription Requests. To exit press CTRL+C")
	Log_debug_event(CommandMessage{}, "Trigger Driver Started")
	<-forever // Block forever to keep the program running while waiting for RPC requests
}
