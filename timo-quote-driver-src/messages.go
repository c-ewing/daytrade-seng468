package main

import "time"

type CommandMessage struct {
	Command           string  `json:"command" bson:"command"`
	TransactionNumber int     `json:"transaction_number,omitempty" bson:"transaction_number,omitempty"`
	Userid            string  `json:"userid,omitempty" bson:"userid,omitempty"`
	StockSymbol       string  `json:"stock_symbol,omitempty" bson:"stock_symbol,omitempty"`
	Amount            float64 `json:"amount,omitempty" bson:"amount,omitempty"`
	Filename          string  `json:"filename,omitempty" bson:"filename,omitempty"`
}

type QuoteReturn struct {
	Command          string    `json:"command" bson:"command"`
	Userid           string    `json:"userid" bson:"userid"`
	StockSymbol      string    `json:"stock_symbol" bson:"stock_symbol"`
	QuotePrice       float64   `json:"quote_price" bson:"quote_price"`
	Timestamp        time.Time `json:"timestamp" bson:"timestamp"`
	CryptographicKey string    `json:"cryptographic_key" bson:"cryptographic_key"`
}
