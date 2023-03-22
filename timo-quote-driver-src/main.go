// TODO: Logging
// TODO: Reduce the number of panic calls
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
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
const QUOTE_SERVER_TIMEOUT_SECONDS = 2

var HOSTNAME = Environment_variable_or_default("DOCKER_HOSTNAME", "QuoteDriver")
var REDIS_CONNECTION_ADDRESS = Environment_variable_or_default("REDIS_CONNECTION_ADDRESS", "quote-price-redis:6379")
var RABBITMQ_CONNECTION_STRING = Environment_variable_or_default("RABBITMQ_CONNECTION_STRING", "amqp://guest:guest@rabbitmq:5672/")
var MONGODB_CONNECTION_STRING = Environment_variable_or_default("MONGODB_CONNECTION_STRING", "mongodb://root:example@mongodb:27017/?retryWrites=true&w=majority")
var QUOTE_SERVER_ADDRESS = Environment_variable_or_default("QUOTE_SERVER_ADDRESS", "quoteserve.seng.uvic.ca:4444")

var MONGO_CLIENT = Connect_mongodb()

// FUNCTIONS:
func main() {
	// This container should be started after the Redis and RabbitMQ containers using a health check

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
		Log_error_event(CommandMessage{}, "Failed to declare a queue: "+err.Error())
		panic(err)
	}

	// Use Quality of Service on the channel to limit the number of messages that can be in flight at once
	// TODO: Add a retry loop here
	err = rabbitmq_channel.Qos(
		1,     // prefetch count (0 = no limit)
		0,     // prefetch size (0 = no limit)
		false, // global (true = apply to entire channel, false = apply to each consumer)
	)

	if err != nil {
		Log_error_event(CommandMessage{}, "Failed to set QoS: "+err.Error())
		panic(err)
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
		Log_error_event(CommandMessage{}, "Failed to register as consumer: "+err.Error())
		panic(err)
	}

	var forever chan struct{}

	// Start a goroutine to handle each message
	go func() {
		// Create a Context that creates a timeout for sending the quote price back to the client
		timeout_context, cancel := context.WithTimeout(context.Background(), RABBITMQ_TIMEOUT_SECONDS*time.Second)
		defer cancel()

		for message := range rabbit_messages {
			// Make sure the message isn't empty
			if message.Body == nil {
				log.Printf(" [error] Received empty message on the quote driver: %+v", message)
				message.Reject(false) // Message is bad, reject is and don't requeue it
				continue
			}

			// Decode the message to a struct
			var command CommandMessage
			err = json.Unmarshal(message.Body, &command)

			// Make sure the message is valid
			if err != nil || command.Command == "" || command.Command != "QUOTE" || command.StockSymbol == "" || command.Userid == "" {
				Log_error_event(command, fmt.Sprintf("Invalid Message received on quote server: %s Error: %s", string(message.Body), err.Error()))
				message.Ack(false) // False = Only acknowledge the current message
				continue
			}

			Log_system_event(command)

			// As more than  one message at a time can sit in the queue using a go routine would add unnecessary complexity
			// If we begin accepting more than one message at a time we would need to add a goroutine to handle each message
			quote := get_or_refresh_quote_price(redis_client, command.StockSymbol, command.Userid)
			Log_quote_server(quote)

			quote_bytes, err := json.Marshal(quote)

			if err != nil {
				Log_error_event(command, "Failed to encode quote return: "+err.Error())
				panic(err)
			}

			// Send the quote price back to the client
			// TODO: Add a retry loop here
			err = rabbitmq_channel.PublishWithContext(
				timeout_context, // context (used to set a timeout)
				"",              // exchange (empty string = default exchange)
				message.ReplyTo, // routing key (the queue to send the RPC response to)
				false,           // mandatory
				false,           // immediate (true = don't send if the queue doesn't exist)

				amqp.Publishing{
					ContentType:   "text/plain",          // Content type of the message, ignored by RabbitMQ
					CorrelationId: message.CorrelationId, // Correlation ID of the message, used to specify who called the RPC
					Body:          quote_bytes,
				},
			)

			if err != nil {
				Log_error_event(command, "Failed to send quote price: "+err.Error())
				panic(err)
			}

			// Acknowledge the message to remove it from the queue now that we have sent the response
			message.Ack(false) // False = Only acknowledge the current message
		}
	}()

	log.Printf(" [info] Waiting for RPC. To exit press CTRL+C")
	Log_debug_event(CommandMessage{}, "Started Quote Driver")
	<-forever // Block forever to keep the program running while waiting for RPC requests
}

// HELPERS:
func get_or_refresh_quote_price(redis_client *redis.Client, symbol string, user string) QuoteReturn {
	// Create a Context that creates a timeout for connecting to Redis
	timeout_context, cancel := context.WithTimeout(context.Background(), REDIS_TIMEOUT_SECONDS*time.Second)
	defer cancel()

	// Query Redis for the quote price
	value_string, err := redis_client.Get(timeout_context, symbol).Result()
	get_new_quote := false

	if err == redis.Nil {
		Log_debug_event(CommandMessage{Command: "QUOTE", Userid: user, StockSymbol: symbol}, "Quote price for "+symbol+" does not exist in Redis, Falling back to Quote Server")
		get_new_quote = true
	} else if err != nil {
		// TODO: Recover from this error, Retry getting the quote price from Redis
		Log_error_event(CommandMessage{Userid: user, StockSymbol: symbol}, "Failed to get quote price for "+symbol+" from Redis: "+err.Error())
		panic(err)
	} else {
		// If the value returned is empty then request the quote price from the Quote server
		if value_string == "" {
			get_new_quote = true
		}
	}

	// Redis has the quote price and it hasn't expired
	if !get_new_quote {
		Log_debug_event(CommandMessage{Command: "QUOTE", Userid: user, StockSymbol: symbol}, "Quote price for "+symbol+" does exist in Redis")
		var quote QuoteReturn
		err = json.Unmarshal([]byte(value_string), &quote)

		if err != nil {
			Log_error_event(CommandMessage{Userid: user, StockSymbol: symbol}, "Failed to decode quote response for "+symbol+" from Redis: "+err.Error())
			panic(err)
		}

		return quote
	}

	// The quote price is not in Redis or it has expired
	// Connect to the Quote server and request it
	value_string = query_quote_server(symbol, user)

	var quote QuoteReturn
	// Parse the response into a QuoteReturn struct
	value_string_split := strings.Split(value_string, ",")
	quote.Command = "QUOTE"
	// Parse the float64 quote price
	quote.QuotePrice, err = strconv.ParseFloat(value_string_split[0], 64)

	if err != nil {
		Log_error_event(CommandMessage{Userid: user, StockSymbol: symbol}, "Failed to parse quote price for "+symbol+" from Quote Server: "+err.Error())
		panic(err)
	}

	quote.StockSymbol = value_string_split[1]
	quote.Userid = value_string_split[2]
	// Parse the int64 timestamp
	timestamp_ms, err := strconv.ParseInt(value_string_split[3], 10, 64)

	if err != nil {
		Log_error_event(CommandMessage{Userid: user, StockSymbol: symbol}, "Failed to parse timestamp for "+symbol+" from Quote Server: "+err.Error())
		panic(err)
	}

	quote.Timestamp = time.UnixMilli(timestamp_ms)
	quote.CryptographicKey = value_string_split[4]

	// Encode the QuoteReturn struct into a JSON string
	value_bytes, err := json.Marshal(quote)

	if err != nil {
		Log_error_event(CommandMessage{Userid: user, StockSymbol: symbol}, "Failed to encode quote return for "+symbol+" from Quote Server: "+err.Error())
		panic(err)
	}

	// QuoteReturn successfully parsed, put it into Redis
	_, err = redis_client.Set(timeout_context, symbol, string(value_bytes), REDIS_EXPIRY_SECONDS*time.Second).Result()

	if err != nil {
		// TODO: Recover from this error, Retry setting the quote price from Redis
		Log_error_event(CommandMessage{Userid: user, StockSymbol: symbol}, "Failed to set quote price for "+symbol+" in Redis: "+err.Error())
		panic(err)
	}

	return quote
}

func query_quote_server(symbol string, user string) string {
	// Connect to the quote server a raw TCP socket
	dialer := net.Dialer{Timeout: QUOTE_SERVER_TIMEOUT_SECONDS * time.Second}
	conn, err := dialer.Dial("tcp", QUOTE_SERVER_ADDRESS)

	if err != nil {
		Log_error_event(CommandMessage{Userid: user, StockSymbol: symbol}, "Failed to connect to quote server: "+err.Error())
		panic(err)
	}

	// Send the symbol and user pair to the quote server
	_, err = conn.Write([]byte(symbol + " " + user))

	if err != nil {
		Log_error_event(CommandMessage{Userid: user, StockSymbol: symbol}, "Failed to send: "+symbol+" "+user+" to quote server: "+err.Error())
		panic(err)
	}

	// Read the response from the quote server
	response := make([]byte, 512) // 512 bytes should be enough to hold the response while still being efficient

	_, err = conn.Read(response)

	if err != nil {
		Log_error_event(CommandMessage{Userid: user, StockSymbol: symbol}, "Failed to read response from quote server: "+err.Error())
		panic(err)
	}

	return string(response)
}
