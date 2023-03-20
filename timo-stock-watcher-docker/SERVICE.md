# Stocker Watcher

Refreshes stock prices for stocks marked as being used for triggered buy and sell actions.

Queries the defined Redis cache for a list of stock symbols being watched and then requests quote updates for them from the *Quote Driver(s)*

This is an **internal** service that has no communication with the end user. This service is used by the *Transaction Server(s)* to trigger buy and sell action for certain set points.

## Broadcast:

This service broadcasts stock price updates to the `stock_price_updates` exchange. Each update uses the stock symbol as the routing key to enable consumers to only subscribe to price updates that are relevant to their functionality.

The broadcast is a simple forwarding of the response from the *Quote Driver(s)* sorted with the correct routing key. As such the broadcast takes the form:

```json
{
    "Command":          "QUOTE",
    "Userid":           string,
    "StockSymbol":      string, // MAX OF 3 CHARACTERS
    "QuotePrice":       float64,
    "Timestamp":        time.Time, // Go Time format
    "CryptographicKey": string,
}
```

## Environment Variables
- `REDIS_CONNECTION_ADDRESS`: URI for the *Trigger Symbol* cache
- `RABBITMQ_CONNECTION_STRING`: URI for the RabbitMQ server

## Interactions:
Used by:
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
