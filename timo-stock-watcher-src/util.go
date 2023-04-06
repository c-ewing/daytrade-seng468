package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DATABASE STRUCTS:
type User struct {
	Userid                      string             `bson:"_id" json:"userid"`
	Account_balance             float64            `bson:"account_balance" json:"account_balance"`
	Account_creation_timestamp  time.Time          `bson:"account_creation_timestamp" json:"account_creation_timestamp"`
	Last_modification_timestamp time.Time          `bson:"last_modification_timestamp" json:"last_modification_timestamp"`
	Owned_stocks                map[string]float64 `bson:"owned_stocks" json:"owned_stocks"`
	Stock_buy_triggers          map[string]Trigger `bson:"stock_buy_triggers" json:"stock_buy_triggers"`
	Stock_sell_triggers         map[string]Trigger `bson:"stock_sell_triggers" json:"stock_sell_triggers"`
}

type Trigger struct {
	Trigger_price  float64 `bson:"trigger_price" json:"trigger_price"`
	Trigger_amount float64 `bson:"trigger_amount" json:"trigger_amount"`
}

type Transaction struct {
	Transaction_number              int64     `bson:"transaction_number" json:"transaction_number"`
	Userid                          string    `bson:"userid" json:"userid"`
	Transaction_start_timestamp     time.Time `bson:"transaction_start_timestamp" json:"transaction_start_timestamp"`
	Transaction_completed_timestamp time.Time `bson:"transaction_completed_timestamp" json:"transaction_completed_timestamp"`
	Transaction_cancelled           bool      `bson:"transaction_cancelled" json:"transaction_cancelled"`
	Transaction_completed           bool      `bson:"transaction_completed" json:"transaction_completed"`
	Transaction_expired             bool      `bson:"transaction_expired" json:"transaction_expired"`
	Transaction_type                string    `bson:"transaction_type" json:"transaction_type"`
	Transaction_amount              float64   `bson:"transaction_amount,omitempty" json:"transaction_amount"`
	Stock_symbol                    string    `bson:"stock_symbol,omitempty" json:"stock_symbol"`
	Stock_units                     float64   `bson:"stock_units,omitempty" json:"stock_units"`
	Trigger_price                   float64   `bson:"trigger_point,omitempty" json:"trigger_point"`
	Quote_price                     float64   `bson:"quote_price,omitempty" json:"quote_price"`
	Quote_timestamp                 time.Time `bson:"quote_timestamp,omitempty" json:"quote_timestamp"`
}

// Environment_variable_or_default returns the value of the environment variable or the default value if it does not exist
func Environment_variable_or_default(key string, def string) string {
	value, exists := os.LookupEnv(key)
	if !exists || value == "" {
		log.Printf(" [warn] Environment variable %s does not exist, using default value: %s", key, def)
		return def
	}
	return value
}

// ### MongoDB ###

// Connect to MongoDB
func Connect_mongodb() *mongo.Client {
	// Use the SetServerAPIOptions() method to set the Stable API version to 1
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(MONGODB_CONNECTION_STRING).SetServerAPIOptions(serverAPI)

	// Create a new mongo_client and connect to the server
	mongo_client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		panic(err)
	}

	return mongo_client
}

// ### RabbitMQ ###

// Connect to RabbitMQ
func Connect_rabbitmq() *amqp.Connection {
	conn, err := amqp.Dial(RABBITMQ_CONNECTION_STRING)

	for i := 0; i < MAX_CONNECTION_RETRIES && err != nil; i++ {
		warn := "Failed to connect to '" + RABBITMQ_CONNECTION_STRING + "', retrying in " + fmt.Sprint(TIME_BETWEEN_RETRIES_SECONDS) + " seconds"
		Log_debug_event(CommandMessage{}, warn)
		time.Sleep(TIME_BETWEEN_RETRIES_SECONDS * time.Second)
		conn, err = amqp.Dial(RABBITMQ_CONNECTION_STRING)
	}

	if err != nil {
		e := "Failed to connect to '" + RABBITMQ_CONNECTION_STRING + "' after " + fmt.Sprint(MAX_CONNECTION_RETRIES) + " retries: " + err.Error()
		Log_error_event(CommandMessage{}, e)
		panic(err)
	}

	return conn
}

// Open a channel to communicate over and retry if it fails
func Open_rabbitmq_channel(rabbit_connection *amqp.Connection) *amqp.Channel {
	channel, err := rabbit_connection.Channel()

	for i := 0; i < MAX_CONNECTION_RETRIES && err != nil; i++ {
		warn := "Failed to open a channel, retrying in " + fmt.Sprint(TIME_BETWEEN_RETRIES_SECONDS) + " seconds"
		Log_debug_event(CommandMessage{}, warn)
		time.Sleep(TIME_BETWEEN_RETRIES_SECONDS * time.Second)
		channel, err = rabbit_connection.Channel()
	}

	if err != nil {
		e := "Failed to open a channel after " + fmt.Sprint(MAX_CONNECTION_RETRIES) + " retries: " + err.Error()
		Log_error_event(CommandMessage{}, e)
		panic(err)
	}

	return channel
}

// ## Generic Helpers ##
// HELPER FUNCTIONS:
func Get_stock_price(symbol string, user string, rabbitmq_channel *amqp.Channel) (quote_return QuoteReturn) {
	// Generate a random correlation ID for the RPC
	corrId := RandomString(32)

	// Timeout context for the RPC
	quote_timeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a queue for the RPC to return to
	rpc_return_queue, err := rabbitmq_channel.QueueDeclare(
		"",    // Auto-generated name
		false, // Durable
		true,  // Delete when unused
		true,  // Exclusive
		false, // No-wait
		nil,   // Arguments
	)

	if err != nil {
		log.Panicf("[error] Failed to declare an RPC return queue: %s", err)
	}

	// Attach a consumer to the return queue
	msgs, err := rabbitmq_channel.Consume(
		rpc_return_queue.Name, // Queue
		"",                    // Consumer
		true,                  // Auto-ack
		false,                 // Exclusive
		false,                 // No-local
		false,                 // No-wait
		nil,                   // Args
	)
	defer rabbitmq_channel.Cancel(rpc_return_queue.Name, true)

	if err != nil {
		log.Panicf("[error] Failed to attach a consumer to the RPC return queue: %s", err)
	}

	// Create a price quote request
	quote_request := CommandMessage{
		Command:     "QUOTE",
		Userid:      user,
		StockSymbol: symbol,
	}

	// JSONify the request
	request_json, err := json.Marshal(quote_request)

	if err != nil {
		log.Panicf("[error] Failed to marshal quote request to JSON: %s", err)
	}

	// Send RPC to quote driver
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
			Body:          request_json,
		},
	)

	if err != nil {
		log.Panicf("[error] Failed to publish a message to the quote_price_requests queue: %s", err)
	}

	// Receive RPC from quote driver
	for message := range msgs {
		if message.CorrelationId == corrId {
			// Parse the quote return
			err := json.Unmarshal(message.Body, &quote_return)

			if err != nil {
				log.Panicf("[error] Failed to unmarshal quote return from JSON: %s", err)
			}

			break
		}
	}
	return
}

func Get_user_account(userid string) (User, error) {
	// Fetch the users current information from the database
	user_accounts := MONGO_CLIENT.Database("users").Collection("accounts")
	filter := bson.D{{Key: "_id", Value: userid}}

	// Query for the user using the user id
	var user User
	err := user_accounts.FindOne(context.Background(), filter).Decode(&user)

	return user, err
}

func Get_last_transaction(userid string, transaction_type string) (Transaction, error) {
	Log_debug_event(CommandMessage{Userid: userid, Command: "DISPLAY_SUMMARY"}, "Fetching last "+transaction_type+" transaction for user; Command is inaccurate, here to satisfy the XSD")
	// Fetch the users current information from the database
	user_transactions := MONGO_CLIENT.Database("users").Collection("transactions")
	filter := bson.M{"userid": userid, "transaction_type": transaction_type, "transaction_completed": false, "transaction_cancelled": false, "transaction_expired": false}
	sort := bson.D{{Key: "transaction_start_timestamp", Value: 1}}
	opts := options.Find().SetSort(sort)

	// Query for the user using the user id
	cursor, err := user_transactions.Find(context.Background(), filter, opts)
	if err != nil {
		e := "Error querying for user " + userid + "'s last transaction: " + err.Error()
		Log_error_event(CommandMessage{Userid: userid}, e)
		return Transaction{}, err
	}

	var transactions []Transaction
	err = cursor.All(context.Background(), &transactions)

	if err != nil {
		e := "Error decoding user " + userid + "'s last transaction: " + err.Error()
		Log_error_event(CommandMessage{Userid: userid}, e)
		return Transaction{}, err
	}

	// Run through all of the transactions and expire them if they are too old
	for i, transaction := range transactions {
		if time.Since(transaction.Transaction_start_timestamp).Seconds() > 60 {
			// Expire the transaction
			transactions[i].Transaction_expired = true
			transactions[i].Transaction_completed = false
			transactions[i].Transaction_cancelled = false
			Upsert_transaction(transactions[i])
			Log_system_event(CommandMessage{Command: "EXPIRE", TransactionNumber: transactions[i].Transaction_number, Userid: transactions[i].Userid, StockSymbol: transactions[i].Stock_symbol, Amount: transactions[i].Transaction_amount})
		}
	}

	// Return the first not expired transaction
	for _, transaction := range transactions {
		if !transaction.Transaction_expired {
			Log_debug_event(CommandMessage{Userid: userid, Command: "DISPLAY_SUMMARY"}, "Returning last "+transaction_type+" transaction for user; Command is inaccurate, here to satisfy the XSD")
			return transaction, nil
		}
	}

	return Transaction{}, err
}

func Upsert_user_account(user User) {
	// Write back to the database
	// We want to either insert or update, MongoDB supports Upsert for this
	options := options.Update().SetUpsert(true)

	user_accounts := MONGO_CLIENT.Database("users").Collection("accounts")
	filter := bson.D{{Key: "_id", Value: user.Userid}}

	_, err := user_accounts.UpdateOne(context.Background(), filter, bson.D{{Key: "$set", Value: user}}, options)

	if err != nil {
		e := "Error updating user " + user.Userid + "'s account: " + err.Error()
		Log_error_event(CommandMessage{Userid: user.Userid}, e)
		panic(err)
	}

}

func Upsert_transaction(transaction Transaction) {
	// Write back to the database
	// We want to either insert or update, MongoDB supports Upsert for this
	options := options.Update().SetUpsert(true)

	transactions := MONGO_CLIENT.Database("users").Collection("transactions")
	filter := bson.D{{Key: "userid", Value: transaction.Userid}, {Key: "transaction_number", Value: transaction.Transaction_number}}

	_, err := transactions.UpdateOne(context.Background(), filter, bson.D{{Key: "$set", Value: transaction}}, options)

	if err != nil {
		e := "Error updating transaction " + transaction.Userid + "," + strconv.Itoa(int(transaction.Transaction_number)) + ": " + err.Error()
		Log_error_event(CommandMessage{Command: "", TransactionNumber: transaction.Transaction_number, Userid: transaction.Userid, StockSymbol: transaction.Stock_symbol, Amount: transaction.Transaction_amount}, e)
		panic(err)
	}

	Log_debug_event(CommandMessage{Command: "", TransactionNumber: transaction.Transaction_number, Userid: transaction.Userid, StockSymbol: transaction.Stock_symbol, Amount: transaction.Transaction_amount}, "Updated/Created transaction: "+strconv.Itoa(int(transaction.Transaction_number)))
}

func RandomString(l int) string {
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(randInt(65, 90))
	}
	return string(bytes)
}

func randInt(min int, max int) int {
	return min + rand.Intn(max-min)
}
