package main

import "time"

type Message struct {
	Command           string  `json:"command" bson:"command"`
	TransactionNumber int     `json:"transaction_number" bson:"transaction_number"`
	Userid            string  `json:"userid" bson:"userid"`
	StockSymbol       string  `json:"stock_symbol" bson:"stock_symbol"`
	Amount            float64 `json:"amount" bson:"amount"`
	Filename          string  `json:"filename" bson:"filename"`
}

type QuoteReturn struct {
	Command          string    `json:"command" bson:"command"`
	Userid           string    `json:"userid" bson:"userid"`
	StockSymbol      string    `json:"stock_symbol" bson:"stock_symbol"`
	QuotePrice       float64   `json:"quote_price" bson:"quote_price"`
	Timestamp        time.Time `json:"timestamp" bson:"timestamp"`
	CryptographicKey string    `json:"cryptographic_key" bson:"cryptographic_key"`
}
