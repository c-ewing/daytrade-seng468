// TODO: Logging
// TODO: Reduce the number of panic calls
package main

import (
	"context"
	"log"
	"math/rand"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	redis "github.com/redis/go-redis/v9"
)

// CONSTANTS:
const MAX_CONNECTION_RETRIES = 5
const TIME_BETWEEN_RETRIES_SECONDS = 5

const TRIGGER_CONNECTION_ADDRESS = "trigger-symbol-redis:6379"
const REDIS_TIMEOUT_SECONDS = 3
const REDIS_EXPIRY_SECONDS = 10

const RABBITMQ_CONNECTION_STRING = "amqp://guest:guest@rabbitmq:5672/"
const RABBITMQ_TIMEOUT_SECONDS = 5

const PRICE_REFRESH_FREQUENCY_SECONDS = 5

// FUNCTIONS:
func main() {
	// This container should be started after Redis
	// However just in case we retry connecting to Redis a few times before giving up

	// First connect to Redis.
	// TODO: Use some sort of password storage solution
	trigger_redis := redis.NewClient(&redis.Options{
		Addr:     TRIGGER_CONNECTION_ADDRESS,                                     // Connection Address
		Password: "timo-daytrade-redispass-8166d5d6-d622-4b62-bfdd-8c7d1a154c2f", // Password
		DB:       0,                                                              // use default DB
	})

	// Connect to RabbitMQ
	rabbit_connection := connect_rabbitmq()
	defer rabbit_connection.Close()
	// Open a channel to communicate over
	rabbitmq_channel := open_channel(rabbit_connection)
	defer rabbitmq_channel.Close()

	// Create a queue if it doesn't exist already to wait for quote price requests
	// TODO: Add a retry loop heres
	rpc_return_queue, err := rabbitmq_channel.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait, don't wait for a response from the server
		nil,   // arguments
	)

	if err != nil {
		log.Panicf("[error] Failed to declare a queue: %s", err)
	}

	// Create a consumer to listen for RPC returns
	msgs, err := rabbitmq_channel.Consume(
		rpc_return_queue.Name, // queue
		"",                    // consumer
		true,                  // auto-ack
		false,                 // exclusive
		false,                 // no-local
		false,                 // no-wait
		nil,                   // args
	)

	// Create a ticker to trigger a refresh every PRICE_REFRESH_FREQUENCY_SECONDS seconds:
	ticker := time.NewTicker(PRICE_REFRESH_FREQUENCY_SECONDS * time.Second)

	// Correlation ID for the RPC calls:
	corrId := randomString(32)

	var forever chan struct{}

	// Start a goroutine to run in the background and refresh the stock prices
	go func() {
		// Refresh the stock prices every PRICE_REFRESH_FREQUENCY_SECONDS seconds
		for range ticker.C {
			// Refresh the stock prices
			log.Printf(" [info] Refreshing Stock Prices: %s", time.Now().Format(time.RFC850))
			refresh_trigger_stock_prices(trigger_redis, rabbitmq_channel, corrId, rpc_return_queue)

			// Clear the RPC return channel after use:
			for len(msgs) > 0 {
				<-msgs
			}
		}
	}()

	log.Printf(" [info] Started Stock Watcher. To exit press CTRL+C")
	<-forever // Block forever to keep the program running
}

// HELPER FUNCTIONS:
func randomString(l int) string {
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(randInt(65, 90))
	}
	return string(bytes)
}

func randInt(min int, max int) int {
	return min + rand.Intn(max-min)
}

func refresh_trigger_stock_prices(trigger_redis *redis.Client, rabbitmq_channel *amqp.Channel, corrId string, rpc_return_queue amqp.Queue) {
	// Create a Context that creates a timeout for updating prices
	trigger_timeout, cancel := context.WithTimeout(context.Background(), REDIS_TIMEOUT_SECONDS*time.Second)
	defer cancel()

	// Get all the symbols that we need to update, this blocks the DB until it returns
	symbols, err := trigger_redis.Keys(trigger_timeout, "*").Result()

	if err != nil {
		log.Panicf("[error] Failed to get symbols from Trigger Symbol Redis: %s", err)
	}

	log.Printf(" [debug] Updating prices for: %s", symbols)

	// Loop through all the symbols and update their prices
	for _, symbol := range symbols {
		// Create a Context that creates a timeout for grabbing a random user
		user_timeout, cancel := context.WithTimeout(context.Background(), REDIS_TIMEOUT_SECONDS*time.Second)
		defer cancel()

		// Get a random username to update the price with (this is a hack to get around the fact that the quote service requires a username)
		user, err := trigger_redis.SRandMember(user_timeout, symbol).Result()

		if err != nil {
			log.Panicf("[error] Failed to get a random user for %s from Trigger Symbol Redis: %s", symbol, err)
		}

		// Update the price for this symbol
		quote_timeout, cancel := context.WithTimeout(context.Background(), RABBITMQ_TIMEOUT_SECONDS*time.Second)
		defer cancel()

		// Send a message to the quote_price_requests queue to get the price for this symbol

		log.Printf(" [info] Updating price for %s using user %s", symbol, user)

		err = rabbitmq_channel.PublishWithContext(
			quote_timeout,
			"",                     // Default Exchange
			"quote_price_requests", // Queue Name
			false,                  // Mandatory
			false,                  // Immediate
			amqp.Publishing{
				ContentType:   "text/plain",
				CorrelationId: corrId,
				ReplyTo:       rpc_return_queue.Name,
				Body:          []byte(symbol + " " + user),
			},
		)

		if err != nil {
			log.Panicf("[error] Failed to publish a message to the quote_price_requests queue: %s", err)
		}
	}
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
