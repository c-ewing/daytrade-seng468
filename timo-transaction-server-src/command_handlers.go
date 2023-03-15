package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// COMMAND FUNCTIONS:

// Add funds to a user's account
// Interactions: MongoDB
func Command_add(command CommandMessage, mongo_client *mongo.Client) string {
	// Check to make sure the command has the correct arguments (Userid + Amount), we already know it is an ADD if it got here
	if command.Userid == "" {
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", command.Amount, "Command ADD missing userid", mongo_client)
		return "error"
	}

	Log_user_command("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", command.Amount, mongo_client)

	// Update the user's account in MongoDB
	update_user := true
	user, err := get_user_account(command.Userid, mongo_client)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No user found, create one
			inf := "Creating new user " + command.Userid
			Log_debug_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", command.Amount, inf, mongo_client)

			// Create a new user
			user = User{
				Userid:                      command.Userid,
				Account_balance:             command.Amount,
				Account_creation_timestamp:  time.Now(),
				Last_modification_timestamp: time.Now(),
				Owned_stocks:                map[string]float64{},
				Stock_buy_triggers:          map[string]float64{},
				Stock_sell_triggers:         map[string]float64{},
			}

			// Set the update_user flag to false so we don't try to update the user if they are newly created
			update_user = false
		} else {
			e := "Error querying database for user " + command.Userid + ": " + err.Error()
			Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", command.Amount, e, mongo_client)
			return "error"
		}
	}

	// Update the user's account balance
	if update_user {
		user.Account_balance += command.Amount
		user.Last_modification_timestamp = time.Now()
	}

	// Write back to the database
	upsert_user_account(user, mongo_client)
	Log_account_transaction("TransactionServer-1", command.TransactionNumber, "ADD", user.Userid, user.Account_balance, mongo_client)

	return "success"
}

// Get a quote for a stock
// Interactions: RabbitMQ (Quote Driver)
func Command_quote(command CommandMessage, mongo_client *mongo.Client, rabbitmq_channel *amqp.Channel) string {
	// Check to make sure the command has the correct arguments (Userid + Stock Symbol)
	if command.Userid == "" || command.StockSymbol == "" {
		e := "Command QUOTE missing userid or stock symbol"
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, command.StockSymbol, "", command.Amount, e, mongo_client)
		return "error"
	}

	Log_user_command("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, command.StockSymbol, "", 0, mongo_client)

	// Get the current stock price from the quote driver
	quote := get_stock_price(command.Userid, command.StockSymbol, rabbitmq_channel)

	// Marshal the quote into a JSON string
	quote_json, err := json.Marshal(quote)
	if err != nil {
		e := "Error marshalling quote: " + err.Error()
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, command.StockSymbol, "", 0, e, mongo_client)
		return "error"
	}

	return string(quote_json)
}

// Buy a stock
// Interactions: MongoDB, RabbitMQ (Quote Driver)
func Command_buy(command CommandMessage, mongo_client *mongo.Client, rabbitmq_channel *amqp.Channel) string {
	// Check to make sure the command has the correct arguments (Userid + Stock Symbol + Amount)
	if command.Userid == "" || command.StockSymbol == "" || command.Amount == 0 {
		e := "Command BUY missing userid, stock symbol, or amount"
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, command.StockSymbol, "", command.Amount, e, mongo_client)
		return "error"
	}

	Log_user_command("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, command.StockSymbol, "", command.Amount, mongo_client)

	// Check if the user exists
	user, err := get_user_account(command.Userid, mongo_client)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No user found, can't buy anything
			warn := "User " + command.Userid + " not found, " + command.Command + "action is invalid unless an account exists"
			Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, command.StockSymbol, "", command.Amount, warn, mongo_client)
			return "error"
		} else {
			e := "Error querying database for user " + command.Userid + ": " + err.Error()
			Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, command.StockSymbol, "", command.Amount, e, mongo_client)
			return "error"
		}
	}

	// Get the current stock price from the quote driver
	quote := get_stock_price(command.Userid, command.StockSymbol, rabbitmq_channel)

	// Create a pending buy transaction in MongoDB
	transaction := Transaction{
		Transaction_number:              rand.Int63(), // TODO: Make this a unique number from the input file / command order
		Userid:                          user.Userid,
		Transaction_start_timestamp:     time.Now(),
		Transaction_completed_timestamp: time.Time{},
		Transaction_cancelled:           false,
		Transaction_completed:           false,
		Transaction_type:                command.Command,
		Transaction_amount:              command.Amount,
		Stock_symbol:                    command.StockSymbol,
		Stock_units:                     command.Amount / quote.QuotePrice,
		Trigger_price:                   -1,
		Quote_price:                     quote.QuotePrice,
		Quote_timestamp:                 quote.Timestamp,
	}

	upsert_transaction(transaction, mongo_client)
	Log_system_event("TransactionServer-1", command.TransactionNumber, command.Command, user.Userid, transaction.Stock_symbol, "", transaction.Transaction_amount, mongo_client)

	return "success"
}

// Confirm the last Buy action
// Interactions: MongoDB
func Command_commit_buy(command CommandMessage, mongo_client *mongo.Client) string {
	// Check to make sure the command has the arguments (Userid)
	if command.Userid == "" {
		e := "Command COMMIT_BUY missing userid"
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, e, mongo_client)
		return "error"
	}

	Log_user_command("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, mongo_client)

	// Get the last BUY transaction for the user
	transaction, err := get_last_transaction(command.Userid, "BUY", mongo_client)

	if err != nil {
		e := "Error querying database for user " + command.Userid + ": " + err.Error()
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, e, mongo_client)
		return "error"
	}

	// Make sure the transaction is not an empty transaction
	if transaction.Userid == "" {
		e := "No pending BUY transaction for user " + command.Userid
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, e, mongo_client)
		return "error"
	}

	// Get the user's account
	user, err := get_user_account(command.Userid, mongo_client)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No user found, can't buy anything
			warn := "User " + command.Userid + " not found, " + command.Command + "action is invalid unless an account exists"
			Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, warn, mongo_client)
			return "error"
		} else {
			e := "Error querying database for user " + command.Userid + ": " + err.Error()
			Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, e, mongo_client)
			return "error"
		}
	}

	if user.Account_balance < transaction.Transaction_amount {
		e := "User " + command.Userid + " does not have enough money to buy " + fmt.Sprintf("%f", transaction.Transaction_amount) + " of " + command.StockSymbol
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, transaction.Stock_symbol, "", transaction.Transaction_amount, e, mongo_client)
		return "error"
	}

	// Subtract the total price from the user's account balance and add the stock to the user's owned stocks
	user.Account_balance -= transaction.Transaction_amount
	user.Owned_stocks[transaction.Stock_symbol] += transaction.Stock_units

	// Update the user's account in MongoDB
	upsert_user_account(user, mongo_client)
	Log_account_transaction("TransactionServer-1", command.TransactionNumber, "REMOVE", user.Userid, transaction.Transaction_amount, mongo_client)
	// Update the transaction in MongoDB
	transaction.Transaction_completed = true
	transaction.Transaction_completed_timestamp = time.Now()
	upsert_transaction(transaction, mongo_client)
	Log_system_event("TransactionServer-1", command.TransactionNumber, command.Command, user.Userid, transaction.Stock_symbol, "", transaction.Transaction_amount, mongo_client)

	return "success"
}

// Cancel the last Buy action
// Interactions: MongoDB
func Command_cancel_buy(command CommandMessage, mongo_client *mongo.Client) string {
	// Check to make sure the command has the correct arguments (Userid)
	if command.Userid == "" {
		e := "Command CANCEL_BUY missing userid"
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, e, mongo_client)
		return "error"
	}

	Log_user_command("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, mongo_client)

	// Get the last BUY transaction for the user
	transaction, err := get_last_transaction(command.Userid, "BUY", mongo_client)

	if err != nil {
		e := "Error querying database for user " + command.Userid + ": " + err.Error()
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, e, mongo_client)
		return "error"
	}

	// Make sure the transaction is not an empty transaction
	if transaction.Userid == "" {
		warn := "No pending BUY transaction for user " + command.Userid
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, warn, mongo_client)
		return "error"
	}

	// Update the transaction in MongoDB
	transaction.Transaction_cancelled = true
	transaction.Transaction_completed_timestamp = time.Now()
	upsert_transaction(transaction, mongo_client)
	Log_system_event("TransactionServer-1", command.TransactionNumber, command.Command, transaction.Userid, transaction.Stock_symbol, "", transaction.Transaction_amount, mongo_client)

	return "success"
}

// Sell a stock
// Interactions: MongoDB, RabbitMQ (Quote Driver)
func Command_sell(command CommandMessage, mongo_client *mongo.Client, rabbitmq_channel *amqp.Channel) string {
	// Check to make sure the command has the correct arguments (Userid + Stock Symbol + Amount)
	if command.Userid == "" || command.StockSymbol == "" || command.Amount == 0 {
		e := "Command SELL missing userid, stock symbol, or amount"
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, command.StockSymbol, "", command.Amount, e, mongo_client)
		return "error"
	}

	Log_user_command("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, command.StockSymbol, "", command.Amount, mongo_client)

	// Check if the user exists
	user, err := get_user_account(command.Userid, mongo_client)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No user found, can't buy anything
			warn := "User " + command.Userid + " not found, " + command.Command + "action is invalid unless an account exists"
			Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, command.StockSymbol, "", command.Amount, warn, mongo_client)
			return "error"
		} else {
			e := "Error querying database for user " + command.Userid + ": " + err.Error()
			Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, command.StockSymbol, "", command.Amount, e, mongo_client)
			return "error"
		}
	}

	// Get the current stock price from the quote driver
	quote := get_stock_price(command.Userid, command.StockSymbol, rabbitmq_channel)

	// Create a pending buy transaction in MongoDB
	transaction := Transaction{
		Transaction_number:              rand.Int63(), // TODO: Make this a unique number from the input file / command order
		Userid:                          user.Userid,
		Transaction_start_timestamp:     time.Now(),
		Transaction_completed_timestamp: time.Time{},
		Transaction_cancelled:           false,
		Transaction_completed:           false,
		Transaction_type:                command.Command,
		Transaction_amount:              command.Amount,
		Stock_symbol:                    command.StockSymbol,
		Stock_units:                     command.Amount / quote.QuotePrice,
		Trigger_price:                   -1,
		Quote_price:                     quote.QuotePrice,
		Quote_timestamp:                 quote.Timestamp,
	}

	upsert_transaction(transaction, mongo_client)
	Log_system_event("TransactionServer-1", command.TransactionNumber, command.Command, transaction.Userid, transaction.Stock_symbol, "", transaction.Transaction_amount, mongo_client)
	return "success"
}

// Confirm the last Sell action
// Interactions: MongoDB
func Command_commit_sell(command CommandMessage, mongo_client *mongo.Client) string {
	// Check to make sure the command has the correct arguments (Userid)
	if command.Userid == "" {
		e := "Command COMMIT_SELL missing userid"
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, e, mongo_client)
		return "error"
	}

	Log_user_command("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, mongo_client)

	// Get the last BUY transaction for the user
	transaction, err := get_last_transaction(command.Userid, "SELL", mongo_client)

	if err != nil {
		e := "Error querying database for user " + command.Userid + ": " + err.Error()
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, e, mongo_client)
		return "error"
	}

	// Make sure the transaction is not an empty transaction
	if transaction.Userid == "" {
		e := "No pending SELL transaction for user " + command.Userid
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, e, mongo_client)
		return "error"
	}

	// Get the user's account
	user, err := get_user_account(command.Userid, mongo_client)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No user found, can't buy anything
			warn := "User " + command.Userid + " not found, " + command.Command + "action is invalid unless an account exists"
			Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, command.StockSymbol, "", 0, warn, mongo_client)
			return "error"
		} else {
			e := "Error querying database for user " + command.Userid + ": " + err.Error()
			Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, command.StockSymbol, "", 0, e, mongo_client)
			return "error"
		}
	}

	if user.Owned_stocks[transaction.Stock_symbol] < transaction.Stock_units {
		e := "User " + command.Userid + " does not have enough stocks to sell " + fmt.Sprintf("%f", transaction.Transaction_amount) + " of " + command.StockSymbol
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, user.Userid, transaction.Stock_symbol, "", transaction.Transaction_amount, e, mongo_client)
		return "error"
	}

	// Subtract the total price from the user's account balance and add the stock to the user's owned stocks
	user.Account_balance += transaction.Transaction_amount
	user.Owned_stocks[transaction.Stock_symbol] -= transaction.Stock_units

	// Update the user's account in MongoDB
	upsert_user_account(user, mongo_client)
	Log_account_transaction("TransactionServer-1", command.TransactionNumber, "ADD", user.Userid, transaction.Transaction_amount, mongo_client)

	// Update the transaction in MongoDB
	transaction.Transaction_completed = true
	transaction.Transaction_completed_timestamp = time.Now()
	upsert_transaction(transaction, mongo_client)
	Log_system_event("TransactionServer-1", command.TransactionNumber, command.Command, transaction.Userid, transaction.Stock_symbol, "", transaction.Transaction_amount, mongo_client)

	return "success"
}

// Cancel the last Sell action
// Interactions: MongoDB
func Command_cancel_sell(command CommandMessage, mongo_client *mongo.Client) string {
	// Check to make sure the command has the correct arguments (Userid)
	if command.Userid == "" {
		e := "Command CANCEL_SELL missing userid"
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, e, mongo_client)
		return "error"
	}

	Log_user_command("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, mongo_client)

	// Get the last SELL transaction for the user
	transaction, err := get_last_transaction(command.Userid, "SELL", mongo_client)

	if err != nil {
		e := "Error querying database for user " + command.Userid + ": " + err.Error()
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, e, mongo_client)
		return "error"
	}

	// Make sure the transaction is not an empty transaction
	if transaction.Userid == "" {
		e := "No pending SELL transaction for user " + command.Userid
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, e, mongo_client)
		return "error"
	}

	// Update the transaction in MongoDB
	transaction.Transaction_cancelled = true
	transaction.Transaction_completed_timestamp = time.Now()
	upsert_transaction(transaction, mongo_client)
	Log_system_event("TransactionServer-1", command.TransactionNumber, command.Command, transaction.Userid, transaction.Stock_symbol, "", transaction.Transaction_amount, mongo_client)

	return "success"
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
func Command_dumplog(command CommandMessage, mongo_client *mongo.Client) (xml []byte) {
	// Check to make sure the command has the correct number of arguments (Filename + [Userid])
	if command.Filename == "" {
		e := "Command DUMPLOG missing filename"
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, "", "", "", 0, e, mongo_client)
		return []byte("error")
	}

	err := error(nil)

	// Get all logs from MongoDB, or just the logs for a specific user
	if command.Userid == "" {
		Log_user_command("TransactionServer-1", command.TransactionNumber, command.Command, "", "", "", 0, mongo_client)
		xml, err = Get_logs("", mongo_client)

		if err != nil {
			e := "Error querying database for logs: " + err.Error()
			Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, "", "", "", 0, e, mongo_client)
			return []byte("error")
		}
		Log_system_event("TransactionServer-1", command.TransactionNumber, command.Command, "", "", "", 0, mongo_client)
	} else {
		Log_user_command("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, mongo_client)
		xml, err = Get_logs(command.Userid, mongo_client)

		if err != nil {
			e := "Error querying database for logs: " + err.Error()
			Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, e, mongo_client)
			return []byte("error")
		}
		Log_system_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, mongo_client)
	}

	return xml
}

// Display a summary of a current user's account
// Interactions: MongoDB
func Command_display_summary(command CommandMessage, mongo_client *mongo.Client) string {
	// Check to make sure the command has the correct number of arguments (Userid)
	if command.Userid == "" {
		e := "Command DISPLAY_SUMMARY missing userid"
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, "", "", "", 0, e, mongo_client)
		return "error"
	}

	Log_user_command("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, mongo_client)

	// Get the user account from MongoDB
	user, err := get_user_account(command.Userid, mongo_client)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No user found, can't display anything
			warn := "User " + command.Userid + " not found, " + command.Command + "action is invalid unless an account exists"
			Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, command.StockSymbol, "", 0, warn, mongo_client)
			return "error"
		} else {
			e := "Error querying database for user " + command.Userid + ": " + err.Error()
			Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, command.StockSymbol, "", 0, e, mongo_client)
			return "error"
		}
	}

	// JSONify the user account
	user_json, err := json.Marshal(user)
	if err != nil {
		e := "Error marshalling user account to JSON: " + err.Error()
		Log_error_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, e, mongo_client)
		return "error"
	}

	Log_system_event("TransactionServer-1", command.TransactionNumber, command.Command, command.Userid, "", "", 0, mongo_client)

	// Return the JSON
	return string(user_json)
}

// HELPER FUNCTIONS:
func get_stock_price(symbol string, user string, rabbitmq_channel *amqp.Channel) (quote_return QuoteReturn) {
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

func get_user_account(userid string, mongo_client *mongo.Client) (User, error) {
	// Fetch the users current information from the database
	user_accounts := mongo_client.Database("users").Collection("accounts")
	filter := bson.D{{Key: "_id", Value: userid}}

	// Query for the user using the user id
	var user User
	err := user_accounts.FindOne(context.Background(), filter).Decode(&user)

	return user, err
}

func get_last_transaction(userid string, transaction_type string, mongo_client *mongo.Client) (Transaction, error) {
	Log_debug_event("TransactionServer-1", -1, "DISPLAY_SUMMARY", userid, "", "", 0, "Fetching last "+transaction_type+" transaction for user; Command is inaccurate, here to satisfy the XSD", mongo_client)
	// Fetch the users current information from the database
	user_transactions := mongo_client.Database("users").Collection("transactions")
	filter := bson.M{"userid": userid, "transaction_type": transaction_type, "transaction_completed": false, "transaction_cancelled": false, "transaction_expired": false}
	sort := bson.D{{Key: "transaction_start_timestamp", Value: 1}}
	opts := options.Find().SetSort(sort)

	// Query for the user using the user id
	cursor, err := user_transactions.Find(context.Background(), filter, opts)
	if err != nil {
		e := "Error querying for user " + userid + "'s last transaction: " + err.Error()
		Log_error_event("TransactionServer-1", -1, "", userid, "", "", 0, e, mongo_client)
		return Transaction{}, err
	}

	var transactions []Transaction
	err = cursor.All(context.Background(), &transactions)

	if err != nil {
		e := "Error decoding user " + userid + "'s last transaction: " + err.Error()
		Log_error_event("TransactionServer-1", -1, "", userid, "", "", 0, e, mongo_client)
		return Transaction{}, err
	}

	// Run through all of the transactions and expire them if they are too old
	for i, transaction := range transactions {
		if time.Since(transaction.Transaction_start_timestamp).Seconds() > 60 {
			// Expire the transaction
			transactions[i].Transaction_expired = true
			transactions[i].Transaction_completed = false
			transactions[i].Transaction_cancelled = false
			upsert_transaction(transactions[i], mongo_client)
			Log_system_event("TransactionServer-1", -1, "EXPIRE", transactions[i].Userid, transactions[i].Stock_symbol, "", transactions[i].Transaction_amount, mongo_client)
		}
	}

	// Return the first not expired transaction
	for _, transaction := range transactions {
		if !transaction.Transaction_expired {
			Log_debug_event("TransactionServer-1", -1, "DISPLAY_SUMMARY", userid, "", "", 0, "Returning last "+transaction_type+" transaction for user; Command is inaccurate, here to satisfy the XSD", mongo_client)
			return transaction, nil
		}
	}

	return Transaction{}, err
}

func upsert_user_account(user User, mongo_client *mongo.Client) {
	// Write back to the database
	// We want to either insert or update, MongoDB supports Upsert for this
	options := options.Update().SetUpsert(true)

	user_accounts := mongo_client.Database("users").Collection("accounts")
	filter := bson.D{{Key: "_id", Value: user.Userid}}

	_, err := user_accounts.UpdateOne(context.Background(), filter, bson.D{{Key: "$set", Value: user}}, options)

	if err != nil {
		e := "Error updating user " + user.Userid + "'s account: " + err.Error()
		Log_error_event("TransactionServer-1", -1, "", user.Userid, "", "", 0, e, mongo_client)
		panic(err)
	}

}

func upsert_transaction(transaction Transaction, mongo_client *mongo.Client) {
	// Write back to the database
	// We want to either insert or update, MongoDB supports Upsert for this
	options := options.Update().SetUpsert(true)

	transactions := mongo_client.Database("users").Collection("transactions")
	filter := bson.D{{Key: "userid", Value: transaction.Userid}, {Key: "transaction_number", Value: transaction.Transaction_number}}

	_, err := transactions.UpdateOne(context.Background(), filter, bson.D{{Key: "$set", Value: transaction}}, options)

	if err != nil {
		e := "Error updating transaction " + transaction.Userid + "," + strconv.Itoa(int(transaction.Transaction_number)) + ": " + err.Error()
		Log_error_event("TransactionServer-1", -1, transaction.Transaction_type, transaction.Userid, transaction.Stock_symbol, "", transaction.Transaction_amount, e, mongo_client)
		panic(err)
	}

	Log_debug_event("TransactionServer-1", -1, transaction.Transaction_type, transaction.Userid, transaction.Stock_symbol, "", transaction.Transaction_amount, "Updated/Created transaction: "+strconv.Itoa(int(transaction.Transaction_number)), mongo_client)
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
	Userid                      string             `bson:"_id" json:"userid"`
	Account_balance             float64            `bson:"account_balance" json:"account_balance"`
	Account_creation_timestamp  time.Time          `bson:"account_creation_timestamp" json:"account_creation_timestamp"`
	Last_modification_timestamp time.Time          `bson:"last_modification_timestamp" json:"last_modification_timestamp"`
	Owned_stocks                map[string]float64 `bson:"owned_stocks" json:"owned_stocks"`
	Stock_buy_triggers          map[string]float64 `bson:"stock_buy_triggers" json:"stock_buy_triggers"`
	Stock_sell_triggers         map[string]float64 `bson:"stock_sell_triggers" json:"stock_sell_triggers"`
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
