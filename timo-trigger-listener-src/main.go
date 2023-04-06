package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	. "r.com/timo"

	"github.com/go-redis/redis"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson"
)

var HOSTNAME = Environment_variable_or_default("DOCKER_HOSTNAME", "TriggerListener")
var TRIGGER_CONNECTION_ADDRESS = Environment_variable_or_default("REDIS_CONNECTION_ADDRESS", "trigger-symbol-redis:6379")
var RABBITMQ_CONNECTION_STRING = Environment_variable_or_default("RABBITMQ_CONNECTION_STRING", "amqp://guest:guest@rabbitmq:5672/")
var MONGODB_CONNECTION_STRING = Environment_variable_or_default("MONGODB_CONNECTION_STRING", "mongodb://root:example@mongodb:27017/?retryWrites=true&w=majority")

// Connect to mongoDB for logging and accessing user accounts
var MONGO_CLIENT = Connect_mongodb(MONGODB_CONNECTION_STRING)

const MONGODB_TIMEOUT_SECONDS = 5
const RABBITMQ_TIMEOUT_SECONDS = 5
const MAX_CONNECTION_RETRIES = 5
const TIME_BETWEEN_RETRIES_SECONDS = 5

func main() {
	// If one of the trigger prices is met, perform the associated action if it is still valid (enough stock/money in the account)
	// Log the action and update the user's account

	// First connect to Redis.
	// TODO: Use some sort of password storage solution
	trigger_redis := redis.NewClient(&redis.Options{
		Addr:     TRIGGER_CONNECTION_ADDRESS,                                     // Connection Address
		Password: "timo-daytrade-redispass-8166d5d6-d622-4b62-bfdd-8c7d1a154c2f", // Password
		DB:       0,                                                              // use default DB
	})

	// Connect to RabbitMQ
	rabbit_connection := Connect_rabbitmq(RABBITMQ_CONNECTION_STRING, TIME_BETWEEN_RETRIES_SECONDS, MAX_CONNECTION_RETRIES)
	defer rabbit_connection.Close()

	// Open a channel to communicate over
	rabbitmq_channel := Open_rabbitmq_channel(rabbit_connection, TIME_BETWEEN_RETRIES_SECONDS, MAX_CONNECTION_RETRIES)
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
		Log_error_event(MONGO_CLIENT, HOSTNAME, CommandMessage{}, fmt.Sprintf("Failed to declare a queue: %s", err))
		panic(err)
	}

	// Create the exchange for stock updates if it doesn't exist:
	err = rabbitmq_channel.ExchangeDeclare(
		"stock_price_updates", // name of the exchange
		"topic",               // type of exchange
		true,                  // durable
		false,                 // delete when complete
		false,                 // internal
		false,                 // no-wait
		nil,                   // arguments
	)

	if err != nil {
		Log_error_event(MONGO_CLIENT, HOSTNAME, CommandMessage{}, fmt.Sprintf("Failed to declare an exchange: %s", err))
		panic(err)
	}

	// Subscribe to stock updates for certain symbols
	// Bind the queue to the exchange:
	// TODO: in the future we will want to bind to specific symbols, rather than all of them
	err = rabbitmq_channel.QueueBind(
		stock_update_queue.Name, // queue name
		"stock.*",               // routing key
		"stock_price_updates",   // exchange name
		false,                   // no wait
		nil,                     // args

	)

	if err != nil {
		Log_error_event(MONGO_CLIENT, HOSTNAME, CommandMessage{}, fmt.Sprintf("Failed to bind a queue: %s", err))
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
		Log_error_event(MONGO_CLIENT, HOSTNAME, CommandMessage{}, fmt.Sprintf("Failed to register a consumer: %s", err))
		panic(err)
	}

	// Start listening for price updates:
	go listen_for_price_updates(msgs, trigger_redis)

	// Log Startup:
	log.Printf(" [*] Waiting for commands. To exit press CTRL+C")
	Log_debug_event(MONGO_CLIENT, HOSTNAME, CommandMessage{}, "Started Trigger Listener")

	// Create a channel to block forever
	var forever chan struct{}
	<-forever
}

// When a stock update is received, get the list of users with triggers on that symbol from trigger-symbols redis cache
func listen_for_price_updates(msgs <-chan amqp.Delivery, trigger_redis *redis.Client) {
	USER_ACCOUNTS := MONGO_CLIENT.Database("users").Collection("accounts")

	for message := range msgs {
		var price_update QuoteReturn
		err := json.Unmarshal(message.Body, &price_update)

		if err != nil {
			Log_error_event(MONGO_CLIENT, HOSTNAME, CommandMessage{}, fmt.Sprintf("Failed to parse price update: %s", err))
			continue
		}

		log.Printf("Received a message: %+v", price_update)

		// Query redis for the subscribed_users subscribed to the symbol
		subscribed_users := trigger_redis.SMembers(price_update.StockSymbol)

		// For each user, check the trigger price and perform a buy/sell if it was met
		for _, user := range subscribed_users.Val() {
			context, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			// Get the user's account from the database
			var user_account User
			err := USER_ACCOUNTS.FindOne(context, bson.M{"username": user}).Decode(&user_account)

			if err != nil {
				Log_error_event(MONGO_CLIENT, HOSTNAME, CommandMessage{}, fmt.Sprintf("Failed to find user account: %s", err))
				cancel()
				continue
			}

			// Check if the user has a sell trigger on this symbol:
			if trigger, ok := user_account.Stock_sell_triggers[price_update.StockSymbol]; ok {
				// User has a sell trigger, check if its met
				if price_update.QuotePrice >= trigger.Trigger_price && trigger.Trigger_price >= 0 {
					// Trigger is met, everything is held in reserve so we can just sell it!
					// TODO: Race condition, if multiple trigger listeners are running then this could occur at the same time on multiple servers.
					// Solved by either partitioning users or stocks across trigger listeners

					user_account.Account_balance += price_update.QuotePrice * trigger.Trigger_amount
					delete(user_account.Stock_sell_triggers, price_update.StockSymbol)

					// Create a psueodo transaction to log this:
					transaction := Transaction{
						Transaction_number:              0,
						Userid:                          user_account.Userid,
						Transaction_start_timestamp:     time.Now(),
						Transaction_completed_timestamp: time.Now(),
						Transaction_completed:           true,
						Transaction_type:                "SELL",
						Transaction_amount:              price_update.QuotePrice * trigger.Trigger_amount,
						Stock_symbol:                    price_update.StockSymbol,
						Stock_units:                     trigger.Trigger_amount,
						Trigger_price:                   trigger.Trigger_price,
						Quote_price:                     price_update.QuotePrice,
						Quote_timestamp:                 price_update.Timestamp,
					}

					// Update and log changes:
					Upsert_user_account(MONGO_CLIENT, HOSTNAME, user_account)
					Log_account_transaction(MONGO_CLIENT, HOSTNAME, transaction.Transaction_number, transaction.Transaction_type, transaction.Userid, transaction.Transaction_amount)
					Upsert_transaction(MONGO_CLIENT, HOSTNAME, transaction)

				}
			}

			// Check if the user has a buy trigger on this symbol:

			cancel()
		}
	}
}
