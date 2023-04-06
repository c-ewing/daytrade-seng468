package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"go.mongodb.org/mongo-driver/mongo"
)

// COMMAND FUNCTIONS:

// Add funds to a user's account
// Interactions: MongoDB
func Command_add(command CommandMessage) string {
	// Check to make sure the command has the correct arguments (Userid + Amount), we already know it is an ADD if it got here
	if command.Userid == "" {
		Log_error_event(command, "Command ADD missing userid")
		return "error"
	}

	// Update the user's account in MongoDB
	update_user := true
	user, err := Get_user_account(command.Userid)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No user found, create one
			inf := "Creating new user " + command.Userid
			Log_debug_event(command, inf)

			// Create a new user
			user = User{
				Userid:                      command.Userid,
				Account_balance:             command.Amount,
				Account_creation_timestamp:  time.Now(),
				Last_modification_timestamp: time.Now(),
				Owned_stocks:                map[string]float64{},
				Stock_buy_triggers:          map[string]Trigger{},
				Stock_sell_triggers:         map[string]Trigger{},
			}

			// Set the update_user flag to false so we don't try to update the user if they are newly created
			update_user = false
		} else {
			e := "Error querying database for user " + command.Userid + ": " + err.Error()
			Log_error_event(command, e)
			return "error"
		}
	}

	// Update the user's account balance
	if update_user {
		user.Account_balance += command.Amount
		user.Last_modification_timestamp = time.Now()
	}

	// Write back to the database
	Upsert_user_account(user)
	Log_account_transaction(command.TransactionNumber, "ADD", user.Userid, user.Account_balance)

	return "success"
}

// Get a quote for a stock
// Interactions: RabbitMQ (Quote Driver)
func Command_quote(command CommandMessage, rabbitmq_channel *amqp.Channel) string {
	// Check to make sure the command has the correct arguments (Userid + Stock Symbol)
	if command.Userid == "" || command.StockSymbol == "" {
		e := "Command QUOTE missing userid or stock symbol"
		Log_error_event(command, e)
		return "error"
	}

	// Get the current stock price from the quote driver
	quote := Get_stock_price(command.Userid, command.StockSymbol, rabbitmq_channel)

	// Marshal the quote into a JSON string
	quote_json, err := json.Marshal(quote)
	if err != nil {
		e := "Error marshalling quote: " + err.Error()
		Log_error_event(command, e)
		return "error"
	}

	return string(quote_json)
}

// Buy a stock
// Interactions: MongoDB, RabbitMQ (Quote Driver)
func Command_buy(command CommandMessage, rabbitmq_channel *amqp.Channel) string {
	// Check to make sure the command has the correct arguments (Userid + Stock Symbol + Amount)
	if command.Userid == "" || command.StockSymbol == "" || command.Amount == 0 {
		e := "Command BUY missing userid, stock symbol, or amount"
		Log_error_event(command, e)
		return "error"
	}

	// Check if the user exists
	user, err := Get_user_account(command.Userid)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No user found, can't buy anything
			warn := "User " + command.Userid + " not found, " + command.Command + "action is invalid unless an account exists"
			Log_error_event(command, warn)
			return "error"
		} else {
			e := "Error querying database for user " + command.Userid + ": " + err.Error()
			Log_error_event(command, e)
			return "error"
		}
	}

	// Get the current stock price from the quote driver
	quote := Get_stock_price(command.Userid, command.StockSymbol, rabbitmq_channel)

	// Create a pending buy transaction in MongoDB
	transaction := Transaction{
		Transaction_number:              command.TransactionNumber,
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

	Upsert_transaction(transaction)
	Log_system_event(command)

	return "success"
}

// Confirm the last Buy action
// Interactions: MongoDB
func Command_commit_buy(command CommandMessage) string {
	// Check to make sure the command has the arguments (Userid)
	if command.Userid == "" {
		e := "Command COMMIT_BUY missing userid"
		Log_error_event(command, e)
		return "error"
	}

	// Get the last BUY transaction for the user
	transaction, err := Get_last_transaction(command.Userid, "BUY")

	if err != nil {
		e := "Error querying database for user " + command.Userid + ": " + err.Error()
		Log_error_event(command, e)
		return "error"
	}

	// Make sure the transaction is not an empty transaction
	if transaction.Userid == "" {
		e := "No pending BUY transaction for user " + command.Userid
		Log_error_event(command, e)
		return "error"
	}

	// Get the user's account
	user, err := Get_user_account(command.Userid)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No user found, can't buy anything
			warn := "User " + command.Userid + " not found, " + command.Command + "action is invalid unless an account exists"
			Log_error_event(command, warn)
			return "error"
		} else {
			e := "Error querying database for user " + command.Userid + ": " + err.Error()
			Log_error_event(command, e)
			return "error"
		}
	}

	if user.Account_balance < transaction.Transaction_amount {
		e := "User " + command.Userid + " does not have enough money to buy " + fmt.Sprintf("%f", transaction.Transaction_amount) + " of " + command.StockSymbol
		Log_error_event(command, e)
		return "error"
	}

	// Subtract the total price from the user's account balance and add the stock to the user's owned stocks
	user.Account_balance -= transaction.Transaction_amount
	user.Owned_stocks[transaction.Stock_symbol] += transaction.Stock_units

	// Update the user's account in MongoDB
	Upsert_user_account(user)
	Log_account_transaction(command.TransactionNumber, "REMOVE", user.Userid, transaction.Transaction_amount)
	// Update the transaction in MongoDB
	transaction.Transaction_completed = true
	transaction.Transaction_completed_timestamp = time.Now()
	Upsert_transaction(transaction)

	Log_system_event(CommandMessage{
		Command:           command.Command,
		TransactionNumber: command.TransactionNumber,
		Userid:            command.Userid,
		StockSymbol:       transaction.Stock_symbol,
		Amount:            transaction.Transaction_amount,
	})

	return "success"
}

// Cancel the last Buy action
// Interactions: MongoDB
func Command_cancel_buy(command CommandMessage) string {
	// Check to make sure the command has the correct arguments (Userid)
	if command.Userid == "" {
		e := "Command CANCEL_BUY missing userid"
		Log_error_event(command, e)
		return "error"
	}

	// Get the last BUY transaction for the user
	transaction, err := Get_last_transaction(command.Userid, "BUY")

	if err != nil {
		e := "Error querying database for user " + command.Userid + ": " + err.Error()
		Log_error_event(command, e)
		return "error"
	}

	// Make sure the transaction is not an empty transaction
	if transaction.Userid == "" {
		warn := "No pending BUY transaction for user " + command.Userid

		Log_error_event(command, warn)
		return "error"
	}

	// Update the transaction in MongoDB
	transaction.Transaction_cancelled = true
	transaction.Transaction_completed_timestamp = time.Now()
	Upsert_transaction(transaction)

	Log_system_event(CommandMessage{
		Command:           command.Command,
		TransactionNumber: command.TransactionNumber,
		Userid:            command.Userid,
		StockSymbol:       transaction.Stock_symbol,
		Amount:            transaction.Transaction_amount,
	})

	return "success"
}

// Sell a stock
// Interactions: MongoDB, RabbitMQ (Quote Driver)
func Command_sell(command CommandMessage, rabbitmq_channel *amqp.Channel) string {
	// Check to make sure the command has the correct arguments (Userid + Stock Symbol + Amount)
	if command.Userid == "" || command.StockSymbol == "" || command.Amount == 0 {
		e := "Command SELL missing userid, stock symbol, or amount"
		Log_error_event(command, e)
		return "error"
	}

	// Check if the user exists
	user, err := Get_user_account(command.Userid)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No user found, can't buy anything
			warn := "User " + command.Userid + " not found, " + command.Command + "action is invalid unless an account exists"
			Log_error_event(command, warn)
			return "error"
		} else {
			e := "Error querying database for user " + command.Userid + ": " + err.Error()
			Log_error_event(command, e)
			return "error"
		}
	}

	// Get the current stock price from the quote driver
	quote := Get_stock_price(command.Userid, command.StockSymbol, rabbitmq_channel)

	// Create a pending buy transaction in MongoDB
	transaction := Transaction{
		Transaction_number:              command.TransactionNumber,
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

	Upsert_transaction(transaction)

	Log_system_event(command)
	return "success"
}

// Confirm the last Sell action
// Interactions: MongoDB
func Command_commit_sell(command CommandMessage) string {
	// Check to make sure the command has the correct arguments (Userid)
	if command.Userid == "" {
		e := "Command COMMIT_SELL missing userid"
		Log_error_event(command, e)
		return "error"
	}

	// Get the last BUY transaction for the user
	transaction, err := Get_last_transaction(command.Userid, "SELL")

	if err != nil {
		e := "Error querying database for user " + command.Userid + ": " + err.Error()
		Log_error_event(command, e)
		return "error"
	}

	// Make sure the transaction is not an empty transaction
	if transaction.Userid == "" {
		e := "No pending SELL transaction for user " + command.Userid
		Log_error_event(command, e)
		return "error"
	}

	// Get the user's account
	user, err := Get_user_account(command.Userid)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No user found, can't buy anything
			warn := "User " + command.Userid + " not found, " + command.Command + "action is invalid unless an account exists"
			Log_error_event(command, warn)
			return "error"
		} else {
			e := "Error querying database for user " + command.Userid + ": " + err.Error()
			Log_error_event(command, e)
			return "error"
		}
	}

	if user.Owned_stocks[transaction.Stock_symbol] < transaction.Stock_units {
		e := "User " + command.Userid + " does not have enough stocks to sell " + fmt.Sprintf("%f", transaction.Transaction_amount) + " of " + command.StockSymbol
		Log_error_event(CommandMessage{
			Command:           command.Command,
			TransactionNumber: command.TransactionNumber,
			Userid:            command.Userid,
			StockSymbol:       transaction.Stock_symbol,
			Amount:            transaction.Transaction_amount,
		}, e)
		return "error"
	}

	// Subtract the total price from the user's account balance and add the stock to the user's owned stocks
	user.Account_balance += transaction.Transaction_amount
	user.Owned_stocks[transaction.Stock_symbol] -= transaction.Stock_units

	// Update the user's account in MongoDB
	Upsert_user_account(user)
	Log_account_transaction(command.TransactionNumber, "ADD", user.Userid, transaction.Transaction_amount)

	// Update the transaction in MongoDB
	transaction.Transaction_completed = true
	transaction.Transaction_completed_timestamp = time.Now()
	Upsert_transaction(transaction)

	Log_system_event(CommandMessage{
		Command:           command.Command,
		TransactionNumber: command.TransactionNumber,
		Userid:            command.Userid,
		StockSymbol:       transaction.Stock_symbol,
		Amount:            transaction.Transaction_amount,
	})

	return "success"
}

// Cancel the last Sell action
// Interactions: MongoDB
func Command_cancel_sell(command CommandMessage) string {
	// Check to make sure the command has the correct arguments (Userid)
	if command.Userid == "" {
		e := "Command CANCEL_SELL missing userid"
		Log_error_event(command, e)
		return "error"
	}

	// Get the last SELL transaction for the user
	transaction, err := Get_last_transaction(command.Userid, "SELL")

	if err != nil {
		e := "Error querying database for user " + command.Userid + ": " + err.Error()
		Log_error_event(command, e)
		return "error"
	}

	// Make sure the transaction is not an empty transaction
	if transaction.Userid == "" {
		e := "No pending SELL transaction for user " + command.Userid
		Log_error_event(command, e)
		return "error"
	}

	// Update the transaction in MongoDB
	transaction.Transaction_cancelled = true
	transaction.Transaction_completed_timestamp = time.Now()
	Upsert_transaction(transaction)

	Log_system_event(CommandMessage{
		Command:           command.Command,
		TransactionNumber: command.TransactionNumber,
		Userid:            command.Userid,
		StockSymbol:       transaction.Stock_symbol,
		Amount:            transaction.Transaction_amount,
	})

	return "success"
}

// Create a triggered buy order
// Interactions: MongoDB, RabbitMQ (Trigger Driver)
func Command_set_buy_amount(command CommandMessage) string {
	log.Println("Unimplemented command: SET_BUY_AMOUNT")
	return "error"
}

// Update a triggered buy order
// Interactions: MongoDB
func Command_set_buy_trigger(command CommandMessage) string {
	log.Println("Unimplemented command: SET_BUY_TRIGGER")
	return "error"
}

// Cancel a triggered buy order
// Interactions: MongoDB, RabbitMQ (Trigger Driver)
func Command_cancel_set_buy(command CommandMessage) string {
	log.Println("Unimplemented command: CANCEL_SET_BUY")
	return "error"
}

// Create a triggered sell order
// Interactions: MongoDB, RabbitMQ (Trigger Driver)
func Command_set_sell_amount(command CommandMessage) string {
	log.Println("Unimplemented command: SET_SELL_AMOUNT")
	return "error"
}

// Update a triggered sell order
// Interactions: MongoDB
func Command_set_sell_trigger(command CommandMessage) string {
	log.Println("Unimplemented command: SET_SELL_TRIGGER")
	return "error"
}

// Cancel a triggered sell order
// Interactions: MongoDB, RabbitMQ (Trigger Driver)
func Command_cancel_set_sell(command CommandMessage) string {
	log.Println("Unimplemented command: CANCEL_SET_SELL")
	return "error"
}

// Dump a logfile, can be for a specific user or all users depending on arguments
// Interactions: MongoDB
func Command_dumplog(command CommandMessage) (xml []byte) {
	// Check to make sure the command has the correct number of arguments (Filename + [Userid])
	if command.Filename == "" {
		e := "Command DUMPLOG missing filename"
		Log_error_event(command, e)
		return []byte("error")
	}

	err := error(nil)

	// Get all logs from MongoDB, or just the logs for a specific user
	if command.Userid == "" {
		xml, err = Get_logs("")

		if err != nil {
			e := "Error querying database for logs: " + err.Error()
			Log_error_event(command, e)
			return []byte("error")
		}
		Log_system_event(command)
	} else {
		xml, err = Get_logs(command.Userid)

		if err != nil {
			e := "Error querying database for logs: " + err.Error()
			Log_error_event(command, e)
			return []byte("error")
		}
		Log_system_event(command)
	}

	return xml
}

// Display a summary of a current user's account
// Interactions: MongoDB
func Command_display_summary(command CommandMessage) string {
	// Check to make sure the command has the correct number of arguments (Userid)
	if command.Userid == "" {
		e := "Command DISPLAY_SUMMARY missing userid"
		Log_error_event(command, e)
		return "error"
	}

	// Get the user account from MongoDB
	user, err := Get_user_account(command.Userid)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// No user found, can't display anything
			warn := "User " + command.Userid + " not found, " + command.Command + "action is invalid unless an account exists"
			Log_error_event(command, warn)
			return "error"
		} else {
			e := "Error querying database for user " + command.Userid + ": " + err.Error()
			Log_error_event(command, e)
			return "error"
		}
	}

	// JSONify the user account
	user_json, err := json.Marshal(user)
	if err != nil {
		e := "Error marshalling user account to JSON: " + err.Error()
		Log_error_event(command, e)
		return "error"
	}

	Log_system_event(command)

	// Return the JSON
	return string(user_json)
}
