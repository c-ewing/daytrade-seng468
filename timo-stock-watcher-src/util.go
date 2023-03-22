package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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
