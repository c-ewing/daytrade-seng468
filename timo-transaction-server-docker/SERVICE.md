# Transaction Server:

Receives user commands and performs the associated actions.

## Supported Commands:
| **Command** | **Parameters** | **Purpose** | **Pre-conditions** | **Post-Conditions** |
|---|---|---|---|---|
| **ADD** | userid, amount | Add the given amount of money to the user's account | none | the user's account is increased by the amount of money specified |
| **QUOTE** | userid,StockSymbol | Get the current quote for the stock for the specified user | none | the current price of the specified stock is displayed to the user |
| **BUY** | userid,StockSymbol,amount | Buy the dollar amount of the stock for the specified user at the current price.  | The user's account must be greater or equal to the amount of the purchase.  | The user is asked to confirm or cancel the transaction |
| **COMMIT_BUY** | userid | Commits the most recently executed BUY command | The user must have executed a BUY command within the previous 60 seconds | (a) the user's cash account is decreased by the amount user to purchase the stock (b) the user's account for the given stock is increased by the purchase amount |
| **CANCEL_BUY** | userid | Cancels the most recently executed BUY Command | The user must have executed a BUY command within the previous 60 seconds | The last BUY command is canceled and any allocated system resources are reset and released. |
| **SELL** | userid,StockSymbol,amount | Sell the specified dollar mount of the stock currently held by the specified user at the current price. | The user's account for the given stock must be greater than or equal to the amount being sold. | The user is asked to confirm or cancel the given transaction |
| **COMMIT_SELL** | userid | Commits the most recently executed SELL command | The user must have executed a SELL command within the previous 60 seconds | (a) the user's account for the given stock is decremented by the sale amount (b) the user's cash account is increased by the sell amount |
| **CANCEL_SELL** | userid | Cancels the most recently executed SELL Command | The user must have executed a SELL command within the previous 60 seconds | The last SELL command is canceled and any allocated system resources are reset and released. |
| **SET_BUY_AMOUNT** | userid,StockSymbol,amount | Sets a defined amount of the given stock to buy when the current stock price is less than or equal to the BUY_TRIGGER | The user's cash account must be greater than or equal to the BUY amount at the time the transaction occurs | (a) a reserve account is created for the BUY transaction to hold the specified amount in reserve for when the transaction is triggered (b) the user's cash account is decremented by the specified amount (c) when the trigger point is reached the user's stock account is updated to reflect the BUYtransaction. |
| **CANCEL_SET_BUY** | userid,StockSymbol | Cancels a SET_BUY command issued for the given stock | The must have been a SET_BUY Command issued for the given stock by the user | (a) All accounts are reset to the values they would have had had the SET_BUY Command not been issued (b) the BUY_TRIGGER for the given user and stock is also canceled. |
| **SET_BUY_TRIGGER** | userid,StockSymbol,amount | Sets the trigger point base on the current stock price when any SET_BUY will execute. | The user must have specified a SET_BUY_AMOUNT prior to setting a SET_BUY_TRIGGER | The set of the user's buy triggers is updated to include the specified trigger |
| **SET_SELL_AMOUNT** | userid,StockSymbol,amount | Sets a defined amount of the specified stock to sell when the current stock price is equal or greater than the sell trigger point  | The user must have the specified amount of stock in their account for that stock. | A trigger is initialized for this username/stock symbol combination, but is not complete until SET_SELL_TRIGGER is executed. |
| **SET_SELL_TRIGGER** | userid,StockSymbol,amount | Sets the stock price trigger point for executing any SET_SELL triggers associated with the given stock and user | The user must have specified a SET_SELL_AMOUNT prior to setting a SET_SELL_TRIGGER | (a) a reserve account is created for the specified amount of the given stock (b) the user account for the given stock is reduced by the max number of stocks that could be purchased and (c) the set of the user's sell triggers is updated to include the specified trigger. |
| **CANCEL_SET_SELL** | userid,StockSymbol | Cancels the SET_SELL associated with the given stock and user | The user must have had a previously set SET_SELL for the given stock | (a) The set of the user's sell triggers is updated to remove the sell trigger associated with the specified stock (b) all user account information is reset to the values they would have been if the given SET_SELL command had not been issued |
| **DUMPLOG** | userid,filename | Print out the history of the users transactions to the user specified file  | none | The history of the user's transaction are written to the specified file. |
| **DUMPLOG** | filename | Print out to the specified file the complete set of transactions that have occurred in the system. | Can only be executed from the supervisor (root/administrator) account. | Places a complete log file of all transactions that have occurred in the system into the file specified by filename |
| **DISPLAY_SUMMARY** | userid | Provides a summary to the client of the given user's transaction history and the current status of their accounts as well as any set buy or sell triggers and their parameters | none | A summary of the given user's transaction history and the current status of their accounts as well as any set buy or sell triggers and their parameters is displayed to the user. |

Taken from the Assignment Command Description

## Command Format:
CommandMessage, as defined within the RabbitMQ SERVICE.md:

```json
 {
	"command":              string,
	"transaction_number":   int64,
	"userid":               string,
	"stock_symbol":         string,		// MAX OF 3 CHARACTERS
	"amount":               float64, 
	"filename":             string,
}
```
Unused fields may be omitted.

Responds with:
TODO: Define a return type to be used in the web interface, talk with Dustin about this

## Environment Variables:
- `MONGODB_CONNECTION_STRING`: MongoDB connection URI for logging
- `RABBITMQ_CONNECTION_STRING`: RabbitMQ connection URI

## Interactions:
Used by:
- User Interface / Website

Depends on:
- Quote Driver
- Stock Watcher
- Trigger Driver
- RabbitMQ
- MongoDB

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