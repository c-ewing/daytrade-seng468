# RabbitMQ:

Acts as the middleware for the application. Handles communication between components as well as routing messages to the relevant services.

This is an **internal** service that has no direct interaction with the user and is isolated to the Docker / Kubernetes environment.

## Queues:

Outside of temporary queues created for RPC return there are several defined queues:

### `command_queue`

This queue is used to send commands to the *Transaction Server(s)* for processing and is defined on the **default** exchange. The queue is `durable`, `not deleted when unused`, `not exclusive`, and waits for the queue to be created on the server (`no wait = false`).

The `command_queue` expects JSON string messages of the form:
```json
 {
	"command":              string,
	"transaction_number":   int,
	"userid":               string,
	"stock_symbol":         string,
	"amount":               float, 
	"filename":             string,
}
```
This is defined as the `CommandMessage` type in messages.go.
All fields are **optional** except for the `command` field.

### `quote_price_requests`

This queue is used to send RPC calls to the *Quote Server(s)* and is defined on the **default** exchange. It is expected that the callee creates a queue for responses to be returned to. The queue is `durable`, `not deleted when unused`, `not exclusive`, and waits for the queue to be created on the server (`no wait = false`).

The `quote_price_requests` queue expects JSON string messages of the form:
```json
 {
	"command":              string,
	"userid":               string,
	"stock_symbol":         string,
}
```
The default `CommandMessage` is accepted as only the three fields listed above are checked and used by the Quote Driver.

## Exchanges:

### `stock_price_updates`

This exchange is used to distribute updates to stock prices that have been marked for watching by the *Trigger Driver(s)*. The *Stock Watcher(s)* microservice publishes updates to this exchange whenever it refreshes a stock price.

This is a `direct` exchange to allow for consumers to filter received messages. The exchanges is defined with the parameters as following: `durable` queue for retaining data over restarts, does not delete on completion (`auto delete = false`), is not internal (`internal - false`), and waits for the queue to be created on the server (`no wait = false`).

The routing keys used on this exchange are `Stock Symbols`.

## Message Formats:

Messages transmitted through RabbitMQ will adhere to one of the following JSON string formats:

### `CommandMessage`

Used to send commands to the *Transaction Server(s)*.
```json
 {
	"command":              string,
	"transaction_number":   int64,
	"userid":               string,
	"stock_symbol":         string,
	"amount":               float64, 
	"filename":             string,
}
```
### `QuoteReturn`

Return by the *Quote Driver(s)* in response to a RPC for a quote price.

```json
{
	"command":				string,
	"userid":				string,
	"stock_symbol":			string,
	"quote_price":			float64,
	"timestamp":			time.Time,
	"cryptographic_key":	string,
}
```