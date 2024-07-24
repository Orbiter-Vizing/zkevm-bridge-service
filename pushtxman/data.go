package pushtxman

import "time"

const TX_STATUS = 2
const OP_SUCCESS = 99

type TxStatus struct {
	Status  string     `json:"status"`
	Message string     `json:"message"`
	Result  ResultData `json:"result"`
}

type ResultData struct {
	ChainId      string    `json:"chainId"`
	Hash         string    `json:"hash"`
	Sender       string    `json:"sender"`
	Receiver     string    `json:"receiver"`
	Amount       string    `json:"amount"`
	Symbol       string    `json:"symbol"`
	Timestamp    time.Time `json:"timestamp"`
	Status       int       `json:"status"`
	OpStatus     int       `json:"opStatus"`
	TargetId     string    `json:"targetId"`
	TargetAmount string    `json:"targetAmount"`
	TargetSymbol string    `json:"targetSymbol"`
	TargetChain  string    `json:"targetChain"`
}
