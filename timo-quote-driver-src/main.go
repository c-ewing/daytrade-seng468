// TODO: Logging
// TODO: Reduce the number of panic calls
package main

import (
	"context"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	redis "github.com/redis/go-redis/v9"
)

// CONSTANTS:
const MAX_CONNECTION_RETRIES = 5
const TIME_BETWEEN_RETRIES_SECONDS = 5

var REDIS_CONNECTION_ADDRESS = environment_variable_or_default("REDIS_CONNECTION_ADDRESS", "quote-price-redis:6379")

const REDIS_TIMEOUT_SECONDS = 3
const REDIS_EXPIRY_SECONDS = 10

var RABBITMQ_CONNECTION_STRING = environment_variable_or_default("RABBITMQ_CONNECTION_STRING", "amqp://guest:guest@rabbitmq:5672/")

const RABBITMQ_TIMEOUT_SECONDS = 5

var QUOTE_SERVER_ADDRESS = environment_variable_or_default("QUOTE_SERVER_ADDRESS", "quoteserve.seng.uvic.ca:4444")

const QUOTE_SERVER_TIMEOUT_SECONDS = 2

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
	rabbit_connection := connect_rabbitmq()
	defer rabbit_connection.Close()
	// Open a channel to communicate over
	rabbitmq_channel := open_channel(rabbit_connection)
	defer rabbitmq_channel.Close()

	// Create an RPC queue if it doesn't exist already to wait for quote price requests
	// TODO: Add a retry loop heres
	rabbit_queue, err := rabbitmq_channel.QueueDeclare( // This has no actual return value
		"quote_price_requests", // name
		true,                   // durable
		false,                  // delete when unused
		false,                  // exclusive
		false,                  // no-wait, don't wait for a response from the server
		nil,                    // arguments
	)

	if err != nil {
		log.Panicf("Failed to declare a queue: %s", err)
	}

	// Use Quality of Service on the channel to limit the number of messages that can be in flight at once
	// TODO: Add a retry loop here
	err = rabbitmq_channel.Qos(
		1,     // prefetch count (0 = no limit)
		0,     // prefetch size (0 = no limit)
		false, // global (true = apply to entire channel, false = apply to each consumer)
	)

	if err != nil {
		log.Panicf("Failed to set QoS: %s", err)
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
		log.Panicf("Failed to register as consumer: %s", err)
	}

	var forever chan struct{}

	// Start a goroutine to handle each message
	go func() {
		// Create a Context that creates a timeout for sending the quote price back to the client
		timeout_context, cancel := context.WithTimeout(context.Background(), RABBITMQ_TIMEOUT_SECONDS*time.Second)
		defer cancel()

		for message := range rabbit_messages {
			split := strings.Split(string(message.Body), " ")
			// Check that the message is valid
			if len(split) != 2 {
				log.Printf(" [warn] Received invalid message: %s", message.Body)
				continue
			}
			symbol, user := split[0], split[1]
			log.Printf(" [info] Received quote request from: %s for %s", user, symbol)

			// As more than  one message at a time can sit in the queue using a go routine would add unnecessary complexity
			// If we begin accepting more than one message at a time we would need to add a goroutine to handle each message
			quote_price := get_or_refresh_quote_price(redis_client, symbol, user)
			log.Printf(" [info] Got quote price: %s for %s for user %s", strconv.FormatFloat(quote_price, 'f', -1, 64), symbol, user)

			// Send the quote price back to the client
			// TODO: Add a retry loop here
			err = rabbitmq_channel.PublishWithContext(
				timeout_context, // context (used to set a timeout)
				"",              // exchange (empty string = default exchange)
				message.ReplyTo, // routing key (the queue to send the RPC response to)
				false,           // mandatory
				false,           // immediate (true = don't send if the queue doesn't exist)

				amqp.Publishing{
					ContentType:   "text/plain",                                                         // Content type of the message, ignored by RabbitMQ
					CorrelationId: message.CorrelationId,                                                // Correlation ID of the message, used to specify who called the RPC
					Body:          []byte(symbol + " " + strconv.FormatFloat(quote_price, 'f', -1, 64)), // Quote price converted to string and then to bytes
				},
			)

			if err != nil {
				log.Panicf(" [error] Failed to send quote price: %s", err)
			}

			// Acknowledge the message to remove it from the queue now that we have sent the response
			message.Ack(false) // False = Only acknowledge the current message
		}
	}()

	log.Printf(" [info] Waiting for RPC. To exit press CTRL+C")
	<-forever // Block forever to keep the program running while waiting for RPC requests
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

func get_or_refresh_quote_price(redis_client *redis.Client, symbol string, user string) float64 {
	// Create a Context that creates a timeout for connecting to Redis
	timeout_context, cancel := context.WithTimeout(context.Background(), REDIS_TIMEOUT_SECONDS*time.Second)
	defer cancel()

	// Query Redis for the quote price
	val, err := redis_client.Get(timeout_context, symbol).Result()
	get_new_quote := false

	if err == redis.Nil {
		log.Printf(" [info] Quote price for %s does not exist in Redis, Falling back to Quote Server", symbol)
		get_new_quote = true
	} else if err != nil {
		// TODO: Recover from this error, Retry getting the quote price from Redis
		log.Panicf(" [error] Failed to get quote price for %s from Redis", symbol)
	} else {
		// If the value returned is empty then request the quote price from the Quote server
		if val == "" {
			get_new_quote = true
		}
	}

	// If it has expired or doesn't exist then connect to the Quote server and request it
	if get_new_quote {
		val = query_quote_server(symbol, user)

		if err != nil {
			log.Panicf(" [error] Failed to connect to quote server: %s", err)
		}

		// Stash it in Redis with an expiry
		val, err = redis_client.Set(timeout_context, symbol, val, REDIS_EXPIRY_SECONDS*time.Second).Result()

		if err != nil {
			// TODO: Recover from this error, Retry setting the quote price from Redis
			log.Panicf(" [error] Failed to set quote price for %s in Redis", symbol)
		}
	}

	// Convert the string to a float
	quote_price, err := strconv.ParseFloat(val, 64)

	if err != nil {
		log.Panicf(" [error] Failed to convert quote price for %s from string to float", val)
	} else {
		log.Printf(" [info] Got quote price: %s for %s from Redis", val, symbol)
	}

	return quote_price
}

func query_quote_server(symbol string, user string) string {
	// Connect to the quote server a raw TCP socket
	dialer := net.Dialer{Timeout: QUOTE_SERVER_TIMEOUT_SECONDS * time.Second}
	conn, err := dialer.Dial("tcp", QUOTE_SERVER_ADDRESS)

	if err != nil {
		log.Panicf(" [error] Failed to connect to quote server: %s", err)
	}

	// Send the symbol and user pair to the quote server
	_, err = conn.Write([]byte(symbol + " " + user))

	if err != nil {
		log.Panicf(" [error] Failed to send: %s to quote server: %s", symbol+" "+user, err)
	}

	// Read the response from the quote server
	response := make([]byte, 512) // 512 bytes should be enough to hold the response while still being efficient

	_, err = conn.Read(response)

	if err != nil {
		log.Panicf(" [error] Failed to read response from quote server: %s", err)
	}

	// Tons of data returned that we don't care about, only care about the quote price
	// The quote price is the first value in the response
	split := strings.Split(string(response), ",")

	return split[0]
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
