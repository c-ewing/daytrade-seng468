package main

import (
	"context"
	"encoding/xml"
	"io/ioutil"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Log levels:
const ERROR = 0
const WARNING = 1
const INFO = 2
const DEBUG = 3

// LOG FUNCTIONS:
func Log_user_command(server string, transaction_num int64, command string, username string, stock_symbol string, filename string, funds float64, mongo_client *mongo.Client) {
	// Form the user command
	user_command := UserCommandType{
		Timestamp:      time.Now().Unix(),
		Server:         server,
		TransactionNum: transaction_num,
		Command:        command,
		Username:       username,
		StockSymbol:    stock_symbol,
		Filename:       filename,
		Funds:          funds,
	}
	// Log to mongodb
	logs := mongo_client.Database("logs").Collection("user_commands")
	_, err := logs.InsertOne(context.Background(), user_command)

	if err != nil {
		log.Panicf(" [error] Error inserting user command into logs: %s", err)
	}
}

func Log_quote_server(server string, transaction_num int64, price float64, stock_symbol string, username string, quote_server_time time.Time, cryptokey string, mongo_client *mongo.Client) {
	// Form the quote server
	quote_server := QuoteServerType{
		Timestamp:       time.Now().Unix(),
		Server:          server,
		TransactionNum:  transaction_num,
		Price:           price,
		StockSymbol:     stock_symbol,
		Username:        username,
		QuoteServerTime: quote_server_time.Unix(),
		Cryptokey:       cryptokey,
	}
	// Log to mongodb
	logs := mongo_client.Database("logs").Collection("quote_server")
	_, err := logs.InsertOne(context.Background(), quote_server)

	if err != nil {
		log.Panicf(" [error] Error inserting quote server into logs: %s", err)
	}
}

func Log_account_transaction(server string, transaction_num int64, action string, username string, funds float64, mongo_client *mongo.Client) {
	// Form the account transaction
	account_transaction := AccountTransactionType{
		Timestamp:      time.Now().Unix(),
		Server:         server,
		TransactionNum: transaction_num,
		Action:         action,
		Username:       username,
		Funds:          funds,
	}
	// Log to mongodb
	logs := mongo_client.Database("logs").Collection("account_transactions")
	_, err := logs.InsertOne(context.Background(), account_transaction)

	if err != nil {
		log.Panicf(" [error] Error inserting account transaction into logs: %s", err)
	}
}

func Log_system_event(server string, transaction_num int64, command string, username string, stock_symbol string, filename string, funds float64, mongo_client *mongo.Client) {
	// Form the system event
	system_event := SystemEventType{
		Timestamp:      time.Now().Unix(),
		Server:         server,
		TransactionNum: transaction_num,
		Command:        command,
		Username:       username,
		StockSymbol:    stock_symbol,
		Filename:       filename,
		Funds:          funds,
	}
	// Log to mongodb
	logs := mongo_client.Database("logs").Collection("system_events")
	_, err := logs.InsertOne(context.Background(), system_event)

	if err != nil {
		log.Panicf(" [error] Error inserting system event into logs: %s", err)
	}
}

func Log_error_event(server string, transaction_num int64, command string, username string, stock_symbol string, filename string, funds float64, error_message string, mongo_client *mongo.Client) {
	// Form the error
	error := ErrorEventType{
		Timestamp:      time.Now().Unix(),
		Server:         server,
		TransactionNum: transaction_num,
		Command:        command,
		Username:       username,
		StockSymbol:    stock_symbol,
		Filename:       filename,
		Funds:          funds,
		ErrorMessage:   error_message,
	}
	// Log to mongodb
	logs := mongo_client.Database("logs").Collection("errors_events")
	_, err := logs.InsertOne(context.Background(), error)

	if err != nil {
		log.Panicf(" [error] Error inserting error into logs: %s", err)
	}
}

func Log_debug_event(server string, transaction_num int64, command string, username string, stock_symbol string, filename string, funds float64, debug_message string, mongo_client *mongo.Client) {
	// Form the debug
	debug := DebugType{
		Timestamp:      time.Now().Unix(),
		Server:         server,
		TransactionNum: transaction_num,
		Command:        command,
		Username:       username,
		StockSymbol:    stock_symbol,
		Filename:       filename,
		Funds:          funds,
		DebugMessage:   debug_message,
	}
	// Log to mongodb
	logs := mongo_client.Database("logs").Collection("debug_events")
	_, err := logs.InsertOne(context.Background(), debug)

	if err != nil {
		log.Panicf(" [error] Error inserting debug into logs: %s", err)
	}
}

func get_user_command_logs(userid string, mongo_client *mongo.Client) (logs []UserCommandType) {
	// Fetch the user_commands logs from the database
	user_logs := mongo_client.Database("logs").Collection("user_commands")
	filter := bson.D{{}}
	if userid != "" {
		filter = bson.D{{Key: "username", Value: userid}}
	}

	// Sort the logs by timestamp
	sort := bson.D{{Key: "timestamp", Value: 1}}
	opts := options.Find().SetSort(sort)

	// Query for the logs
	cursor, err := user_logs.Find(context.Background(), filter, opts)
	if err != nil {
		log.Panicf(" [error] Error querying for logs: %s", err)
	}

	err = cursor.All(context.Background(), &logs)

	if err != nil {
		log.Panicf(" [error] Error decoding logs: %s", err)
	}
	return logs
}

func get_quote_server_logs(userid string, mongo_client *mongo.Client) (logs []QuoteServerType) {
	// Fetch the quote_server logs from the database
	quote_server_logs := mongo_client.Database("logs").Collection("quote_server")
	filter := bson.D{{}}
	if userid != "" {
		filter = bson.D{{Key: "username", Value: userid}}
	}

	// Sort the logs by timestamp
	sort := bson.D{{Key: "timestamp", Value: 1}}
	opts := options.Find().SetSort(sort)

	// Query for the logs
	cursor, err := quote_server_logs.Find(context.Background(), filter, opts)
	if err != nil {
		log.Panicf(" [error] Error querying for logs: %s", err)
	}

	err = cursor.All(context.Background(), &logs)

	if err != nil {
		log.Panicf(" [error] Error decoding logs: %s", err)
	}
	return logs
}

func get_account_transaction_logs(userid string, mongo_client *mongo.Client) (logs []AccountTransactionType) {
	// Fetch the account_transaction logs from the database
	account_transaction_logs := mongo_client.Database("logs").Collection("account_transactions")
	filter := bson.D{{}}
	if userid != "" {
		filter = bson.D{{Key: "username", Value: userid}}
	}

	// Sort the logs by timestamp
	sort := bson.D{{Key: "timestamp", Value: 1}}
	opts := options.Find().SetSort(sort)

	// Query for the logs
	cursor, err := account_transaction_logs.Find(context.Background(), filter, opts)
	if err != nil {
		log.Panicf(" [error] Error querying for logs: %s", err)
	}

	err = cursor.All(context.Background(), &logs)

	if err != nil {
		log.Panicf(" [error] Error decoding logs: %s", err)
	}
	return logs
}

func get_system_event_logs(userid string, mongo_client *mongo.Client) (logs []SystemEventType) {
	// Fetch the system_event logs from the database
	system_event_logs := mongo_client.Database("logs").Collection("system_events")
	filter := bson.D{{}}
	if userid != "" {
		filter = bson.D{{Key: "username", Value: userid}}
	}

	// Sort the logs by timestamp
	sort := bson.D{{Key: "timestamp", Value: 1}}
	opts := options.Find().SetSort(sort)

	// Query for the logs
	cursor, err := system_event_logs.Find(context.Background(), filter, opts)
	if err != nil {
		log.Panicf(" [error] Error querying for logs: %s", err)
	}

	err = cursor.All(context.Background(), &logs)

	if err != nil {
		log.Panicf(" [error] Error decoding logs: %s", err)
	}
	return logs
}

func get_error_event_logs(userid string, mongo_client *mongo.Client) (logs []ErrorEventType) {
	// Fetch the error logs from the database
	error_logs := mongo_client.Database("logs").Collection("error_events")
	filter := bson.D{{}}
	if userid != "" {
		filter = bson.D{{Key: "username", Value: userid}}
	}

	// Sort the logs by timestamp
	sort := bson.D{{Key: "timestamp", Value: 1}}
	opts := options.Find().SetSort(sort)

	// Query for the logs
	cursor, err := error_logs.Find(context.Background(), filter, opts)
	if err != nil {
		log.Panicf(" [error] Error querying for logs: %s", err)
	}

	err = cursor.All(context.Background(), &logs)

	if err != nil {
		log.Panicf(" [error] Error decoding logs: %s", err)
	}
	return logs
}

func get_debug_event_logs(userid string, mongo_client *mongo.Client) (logs []DebugType) {
	// Fetch the error logs from the database
	error_logs := mongo_client.Database("logs").Collection("debug_events")
	filter := bson.D{{}}
	if userid != "" {
		filter = bson.D{{Key: "username", Value: userid}}
	}

	// Sort the logs by timestamp
	sort := bson.D{{Key: "timestamp", Value: 1}}
	opts := options.Find().SetSort(sort)

	// Query for the logs
	cursor, err := error_logs.Find(context.Background(), filter, opts)
	if err != nil {
		log.Panicf(" [error] Error querying for logs: %s", err)
	}

	err = cursor.All(context.Background(), &logs)

	if err != nil {
		log.Panicf(" [error] Error decoding logs: %s", err)
	}
	return logs
}

func Get_logs(userid string, mongo_client *mongo.Client) ([]byte, error) {
	// Fetch the logs from the database
	user_logs := get_user_command_logs(userid, mongo_client)
	quote_server_logs := get_quote_server_logs(userid, mongo_client)
	account_transaction_logs := get_account_transaction_logs(userid, mongo_client)
	system_event_logs := get_system_event_logs(userid, mongo_client)
	error_event_logs := get_error_event_logs(userid, mongo_client)
	debug_event_logs := get_debug_event_logs(userid, mongo_client)

	// Assemble the logs
	log_container := Log{
		UserCommand:        user_logs,
		QuoteServer:        quote_server_logs,
		AccountTransaction: account_transaction_logs,
		SystemEvent:        system_event_logs,
		ErrorEvent:         error_event_logs,
		DebugEvent:         debug_event_logs,
	}

	// Convert the logs to XML
	xml, err := xml.MarshalIndent(log_container, "", "  ")
	if err != nil {
		return nil, err
	}

	// TODO: REMOVE THIS
	// Write logs to file
	_ = ioutil.WriteFile("logs.xml", xml, 0644)

	return xml, nil
}
