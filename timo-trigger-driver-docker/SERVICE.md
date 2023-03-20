# Trigger Driver

Used to mark stock as being watched for triggered buy and sell actions. 

This is an **internal** service that has no communication with the end user. This service is used by the *Transaction Server(s)* to set stcoks as watched for triggered buy and sell actions.

## Usage:

On receiving a `TRIGGER_ADD` command, adds the stock symbol and user to the list of watched stocks in the *Trigger Symbol* cache. The command is of form:
```json
{
    "Command":          "TRIGGER_ADD",
    "Userid":           string,
    "StockSymbol":      string, // MAX OF 3 CHARACTERS
}
```

On receiving a `TRIGGER_REMOVE` command, removes the stock symbol and user from the list of watched stocks in the *Trigger Symbol* cache. The command is of form:
```json
{
    "Command":          "TRIGGER_REMOVE",
    "Userid":           string,
    "StockSymbol":      string, // MAX OF 3 CHARACTERS
}
```

## Environment Variables
- `REDIS_CONNECTION_ADDRESS`: URI for the *Trigger Symbol* cache
- `RABBITMQ_CONNECTION_STRING`: URI for the RabbitMQ server

## Interactions:
Used by:
- Transaction Server

Depended on By:
- Stock Watcher

## Libraries:
---
Logging: `logging.go`

Used for logging to the MongoDB Database
- Depends on: `xml_structs.go`
- Depends on: `mongoDB.go`
*TODO: mongoDB.go needs to be refactored out of all the microservices*

---
RabbitMQ: `rabbitMQ.go`

Middleware and used for communicating with other services
*TODO: This needs to be refactored out of all the microservices*

---
Commands: `messages.go`

Defines the possible messages sent via RabbitMQ. Enables conversion to/from JSON strings (message format) to Go structs for use within code.
