package main

import (
	"context"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// COMMAND FUNCTIONS:

// Add funds to a user's account
// Interactions: MongoDB
func Command_add(command_arguments []string, mongo_client *mongo.Client) string {
	// Check to make sure the command has the correct number of arguments (Command + Userid + Amount)
	if len(command_arguments) != 3 {
		log.Printf("Invalid number of arguments for ADD command: %s", command_arguments)
		return "error"
	}

	// Parse the amount to add to the account
	amount, err := strconv.ParseFloat(command_arguments[2], 64)
	if err != nil {
		log.Printf("Error parsing amount to add to account: %s", err)
		return "error"
	}

	// Update the user's account in MongoDB
	update_user := true
	user, err := get_user_account(command_arguments[1], mongo_client)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No user found, create one
			log.Printf(" [info] User %s not found, creating a new account", command_arguments[1])

			// Create a new user
			user = User{
				Userid:                      command_arguments[1],
				Account_balance:             amount,
				Account_creation_timestamp:  time.Now(),
				Last_modification_timestamp: time.Now(),
				Owned_stocks:                map[string]float64{},
				Stock_buy_triggers:          map[string]float64{},
				Stock_sell_triggers:         map[string]float64{},
			}

			// Set the update_user flag to false so we don't try to update the user if they are newly created
			update_user = false
		} else {
			log.Printf("Error querying database for user %s: %s", command_arguments[1], err)
			return "error"
		}
	}

	// Update the user's account balance
	if update_user {
		user.Account_balance += amount
		user.Last_modification_timestamp = time.Now()
	}

	// Write back to the database
	upsert_user_account(user, mongo_client)

	log.Printf(" [info] Added $%f to user %s's account", amount, command_arguments[1])
	return "success"
}

// Get a quote for a stock
// Interactions: RabbitMQ (Quote Driver)
func Command_quote(command_arguments []string, rabbitmq_channel *amqp.Channel) string {
	// Check to make sure the command has the correct number of arguments (Command + Stock Symbol)
	if len(command_arguments) != 3 {
		log.Printf("Invalid number of arguments for QUOTE command: %s", command_arguments)
		return "error"
	}

	// Get the current stock price from the quote driver
	stock_price := get_stock_price(command_arguments[1], command_arguments[2], rabbitmq_channel)

	return strconv.FormatFloat(stock_price, 'f', -1, 64)
}

// Buy a stock
// Interactions: MongoDB, RabbitMQ (Quote Driver)
func Command_buy(command_arguments []string, mongo_client *mongo.Client, rabbitmq_channel *amqp.Channel) {
	// Check to make sure the command has the correct number of arguments (Command + Stock Symbol + Amount)
	if len(command_arguments) != 4 {
		log.Printf("Invalid number of arguments for BUY command: %s", command_arguments)
		return
	}

	// Parse the buy_amount to buy
	buy_amount, err := strconv.ParseFloat(command_arguments[3], 64)
	if err != nil {
		log.Panicf("Error parsing amount to buy: %s", err)
	}

	// Check if the user has enough money to buy the stock
	user, err := get_user_account(command_arguments[1], mongo_client)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No user found, can't buy anything
			log.Printf(" [warn] User %s not found, %s action is invalid unless an account exists", command_arguments[1], command_arguments[0])
		} else {
			log.Printf("Error querying database for user %s: %s", command_arguments[1], err)
			return
		}
	}

	if user.Account_balance < buy_amount {
		log.Printf(" [warn] User %s does not have enough money to buy %f$ of %s", command_arguments[1], buy_amount, command_arguments[2])
		return
	}

	// Get the current stock price from the quote driver
	stock_price := get_stock_price(command_arguments[1], command_arguments[2], rabbitmq_channel)
	// Subtract the total price from the user's account balance and add the stock to the user's owned stocks
	user.Account_balance -= buy_amount
	user.Owned_stocks[command_arguments[2]] += buy_amount / stock_price

	// TODO: Log the transaction in mongo

	// Update the user's account in MongoDB
	upsert_user_account(user, mongo_client)
}

// Confirm the last Buy action
// Interactions: MongoDB
func Command_commit_buy() {
	log.Println("Unimplemented command: COMMIT_BUY")
}

// Cancel the last Buy action
// Interactions: MongoDB
func Command_cancel_buy() {
	log.Println("Unimplemented command: CANCEL_BUY")
}

// Sell a stock
// Interactions: MongoDB, RabbitMQ (Quote Driver)
func Command_sell() {
	log.Println("Unimplemented command: SELL")
}

// Confirm the last Sell action
// Interactions: MongoDB
func Command_commit_sell() {
	log.Println("Unimplemented command: COMMIT_SELL")
}

// Cancel the last Sell action
// Interactions: MongoDB
func Command_cancel_sell() {
	log.Println("Unimplemented command: CANCEL_SELL")
}

// Create a triggered buy order
// Interactions: MongoDB, RabbitMQ (Trigger Driver)
func Command_set_buy_amount() {
	log.Println("Unimplemented command: SET_BUY_AMOUNT")
}

// Update a triggered buy order
// Interactions: MongoDB
func Command_set_buy_trigger() {
	log.Println("Unimplemented command: SET_BUY_TRIGGER")
}

// Cancel a triggered buy order
// Interactions: MongoDB, RabbitMQ (Trigger Driver)
func Command_cancel_set_buy() {
	log.Println("Unimplemented command: CANCEL_SET_BUY")
}

// Create a triggered sell order
// Interactions: MongoDB, RabbitMQ (Trigger Driver)
func Command_set_sell_amount() {
	log.Println("Unimplemented command: SET_SELL_AMOUNT")
}

// Update a triggered sell order
// Interactions: MongoDB
func Command_set_sell_trigger() {
	log.Println("Unimplemented command: SET_SELL_TRIGGER")
}

// Cancel a triggered sell order
// Interactions: MongoDB, RabbitMQ (Trigger Driver)
func Command_cancel_set_sell() {
	log.Println("Unimplemented command: CANCEL_SET_SELL")
}

// Dump a logfile, can be for a specific user or all users depending on arguments
// Interactions: MongoDB
func Command_dumplog() {
	log.Println("Unimplemented command: DUMPLOG")
}

// Display a summary of a current user's account
// Interactions: MongoDB
func Command_display_summary() {
	log.Println("Unimplemented command: DISPLAY_SUMMARY")
}

// HELPER FUNCTIONS:
func get_stock_price(symbol string, user string, rabbitmq_channel *amqp.Channel) (price float64) {
	// Generate a random correlation ID for the RPC
	corrId := randomString(32)

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
			Body:          []byte(symbol + " " + user),
		},
	)

	if err != nil {
		log.Panicf("[error] Failed to publish a message to the quote_price_requests queue: %s", err)
	}

	// Receive RPC from quote driver
	for message := range msgs {
		if message.CorrelationId == corrId {
			// Parse the price from the message
			_, price = parse_rpc_return(message.Body)
			break
		}
	}
	return price
}

func get_user_account(userid string, mongo_client *mongo.Client) (User, error) {
	// Fetch the users current information from the database
	user_accounts := mongo_client.Database("users").Collection("accounts")
	filter := bson.D{{Key: "_id", Value: userid}}

	// Query for the user using the user id
	var user User
	err := user_accounts.FindOne(context.Background(), filter).Decode(&user)

	return user, err
}

func upsert_user_account(user User, mongo_client *mongo.Client) {
	// Write back to the database
	// We want to either insert or update, MongoDB supports Upsert for this
	options := options.Update().SetUpsert(true)

	user_accounts := mongo_client.Database("users").Collection("accounts")
	filter := bson.D{{Key: "_id", Value: user.Userid}}

	_, err := user_accounts.UpdateOne(context.Background(), filter, bson.D{{Key: "$set", Value: user}}, options)

	if err != nil {
		log.Panicf("Error updating user %s's account: %s", user.Userid, err)
	}

	log.Printf(" [info] Updated/Created user: %s ", user.Userid)
}

func parse_rpc_return(body []byte) (string, float64) {
	s := strings.Split(string(body), " ")
	if len(s) != 2 {
		log.Panicf("[error] Failed to parse RPC return: %s", body)
	}

	symbol := s[0]
	price, err := strconv.ParseFloat(s[1], 64)

	if err != nil {
		log.Panicf("[error] Failed to parse price: %s", err)
	}

	return symbol, price
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

// DATABASE STRUCTS:
type User struct {
	Userid                      string             `bson:"_id"`
	Account_balance             float64            `bson:"account_balance"`
	Account_creation_timestamp  time.Time          `bson:"account_creation_timestamp"`
	Last_modification_timestamp time.Time          `bson:"last_modification_timestamp"`
	Owned_stocks                map[string]float64 `bson:"owned_stocks"`
	Stock_buy_triggers          map[string]float64 `bson:"stock_buy_triggers"`
	Stock_sell_triggers         map[string]float64 `bson:"stock_sell_triggers"`
}
