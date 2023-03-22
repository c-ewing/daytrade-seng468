// TODO: Logging
// TODO: Reduce the number of panic calls
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	redis "github.com/redis/go-redis/v9"
)

// CONSTANTS:
const MAX_CONNECTION_RETRIES = 5
const TIME_BETWEEN_RETRIES_SECONDS = 5
const REDIS_TIMEOUT_SECONDS = 3
const REDIS_EXPIRY_SECONDS = 10
const RABBITMQ_TIMEOUT_SECONDS = 5

const PRICE_REFRESH_FREQUENCY_SECONDS = 5

var HOSTNAME = Environment_variable_or_default("DOCKER_HOSTNAME", "TriggerListener")
var TRIGGER_CONNECTION_ADDRESS = Environment_variable_or_default("REDIS_CONNECTION_ADDRESS", "trigger-symbol-redis:6379")
var RABBITMQ_CONNECTION_STRING = Environment_variable_or_default("RABBITMQ_CONNECTION_STRING", "amqp://guest:guest@rabbitmq:5672/")
var MONGODB_CONNECTION_STRING = Environment_variable_or_default("MONGODB_CONNECTION_STRING", "mongodb://root:example@mongodb:27017/?retryWrites=true&w=majority")

var MONGO_CLIENT = Connect_mongodb()

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
	rabbit_connection := Connect_rabbitmq()
	defer rabbit_connection.Close()
	// Open a channel to communicate over
	rabbitmq_channel := Open_rabbitmq_channel(rabbit_connection)
	defer rabbitmq_channel.Close()

	// Create a queue if it doesn't exist already
	// TODO: Add a retry loop here
	rpc_return_queue, err := rabbitmq_channel.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait, don't wait for a response from the server
		nil,   // arguments
	)

	if err != nil {
		Log_error_event(CommandMessage{}, fmt.Sprintf("Failed to declare a queue: %s", err))
		panic(err)
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

	if err != nil {
		Log_error_event(CommandMessage{}, fmt.Sprintf("Failed to register a consumer: %s", err))
		panic(err)
	}

	// Create a new Exchange to broadcast stock price updates
	// TODO: Add a retry loop here
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
			// log.Printf(" [info] Refreshing Stock Prices: %s", time.Now().Format(time.RFC850))
			refresh_trigger_stock_prices(trigger_redis, rabbitmq_channel, corrId, rpc_return_queue)
			// log.Printf(" [info] Finished Refreshing Stock Prices: %s", time.Now().Format(time.RFC850))
		}
	}()

	go price_broadcast(corrId, rabbitmq_channel, msgs)

	log.Printf(" [info] Started Stock Watcher. To exit press CTRL+C")
	Log_debug_event(CommandMessage{}, "Started Stock Watcher")
	<-forever // Block forever to keep the program running
}

// HELPER FUNCTIONS:
func price_broadcast(corrId string, rabbitmq_channel *amqp.Channel, msgs <-chan amqp.Delivery) {
	for message := range msgs {
		if corrId != message.CorrelationId {
			message.Reject(true)
			continue
		}

		// Parse the RPC return
		var rpc_return QuoteReturn
		err := json.Unmarshal(message.Body, &rpc_return)

		if err != nil {
			Log_error_event(CommandMessage{}, fmt.Sprintf("Failed to parse RPC return: %s", err))
			panic(err)
		}

		// log.Printf(" [info] Broadcasting Stock Price Update: %s is %f", rpc_return.StockSymbol, rpc_return.QuotePrice)

		// Create a context that times out after RABBITMQ_TIMEOUT_SECONDS seconds
		rabbitmq_timeout, cancel := context.WithTimeout(context.Background(), RABBITMQ_TIMEOUT_SECONDS*time.Second)

		// Broadcast the stock price update to the exchange
		err = rabbitmq_channel.PublishWithContext(
			rabbitmq_timeout,       // context
			"stock_price_updates",  // exchange
			rpc_return.StockSymbol, // routing key
			false,                  // mandatory
			false,                  // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        message.Body, // Use the same message body as the RPC return, basically just sorting into the correct queue
			},
		)

		if err != nil {
			Log_error_event(CommandMessage{}, fmt.Sprintf("Failed to publish triggered stock price update message: %s", err))
			panic(err)
		}

		cancel()
	}
}

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
		Log_error_event(CommandMessage{}, fmt.Sprintf("Failed to get symbols from Trigger Symbol Redis: %s", err))
		panic(err)
	}

	// Loop through all the symbols and update their prices
	for _, symbol := range symbols {
		// Create a Context that creates a timeout for grabbing a random user
		user_timeout, cancel := context.WithTimeout(context.Background(), REDIS_TIMEOUT_SECONDS*time.Second)
		defer cancel()

		// Get a random username to update the price with (this is a hack to get around the fact that the quote service requires a username)
		user, err := trigger_redis.SRandMember(user_timeout, symbol).Result()

		if err != nil {
			Log_error_event(CommandMessage{}, fmt.Sprintf("Failed to get a random user for %s from Trigger Symbol Redis: %s", symbol, err))
			panic(err)
		}

		// Update the price for this symbol
		quote_timeout, cancel := context.WithTimeout(context.Background(), RABBITMQ_TIMEOUT_SECONDS*time.Second)

		// Send a message to the quote_price_requests queue to get the price for this symbol

		// log.Printf(" [info] Updating price for %s using user %s", symbol, user)

		// Create a new quote request message:
		price_request := CommandMessage{
			Command:     "QUOTE",
			Userid:      user,
			StockSymbol: symbol,
		}
		// Convert to JSON byte slice for sending over RabbitMQ
		price_request_json, err := json.Marshal(price_request)

		if err != nil {
			Log_error_event(CommandMessage{}, fmt.Sprintf("Failed to convert quote request to JSON: %s", err))
			panic(err)
		}

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
				Body:          price_request_json,
			},
		)

		if err != nil {
			Log_error_event(CommandMessage{}, fmt.Sprintf("Failed to publish a message to the quote_price_requests queue: %s", err))
			panic(err)
		}
		cancel()
	}
	Log_debug_event(CommandMessage{}, fmt.Sprintf("Refreshed Stock Prices for %s", symbols))

}
