# Quote Driver

Receives RPC calls through RabbitMQ messages for price quotes for a specified stock symbol for a given user. Returns the quoted price for the specified stock and user to the caller by a caller created RabbitMQ response queue.

This is an **internal** service that has no communication with the end user directly. This service is used by the `Transaction Server` as well as the `Stock Watcher` microservice

## Call Format:
Listens for a serialized JSON string message, containing a command of the format:
```json
{
    "Command":      "QUOTE",
    "Userid":       string,
    "StockSymbol":  string, // MAX OF 3 CHARACTERS
}

Responds with:
{
    "Command":          "QUOTE",
    "Userid":           string,
    "StockSymbol":      string, // MAX OF 3 CHARACTERS
    "QuotePrice":       float64,
    "Timestamp":        time.Time, // Go Time format
    "CryptographicKey": string,
}
```

## Environment Variables:
- `REDIS_CONNECTION_ADDRESS`: Redis connection URI for the *Quote Cache*
- `RABBITMQ_CONNECTION_STRING`: RabbitMQ connection URI
- `QUOTE_SERVER_ADDRESS`: Connection URL for the *Quote Server*

## Interactions:
Used by:
- Stock Watcher
- Transaction Server

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
