package server

import (
	"strconv"
	"strings"
	"time"
)

type HistoryRes struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  Result `json:"result"`
}

type Result struct {
	Count  uint64 `json:"count"`
	Rows   []Row  `json:"rows"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

type Row struct {
	SourceId      string    `json:"sourceId"`
	TargetId      string    `json:"targetId"`
	SourceChain   string    `json:"sourceChain"`
	TargetChain   string    `json:"targetChain"`
	SourceAmount  string    `json:"sourceAmount"`
	SourceMaker   string    `json:"sourceMaker"`
	SourceAddress string    `json:"sourceAddress"`
	TargetAddress string    `json:"targetAddress"`
	SourceSymbol  string    `json:"sourceSymbol"`
	TargetSymbol  string    `json:"targetSymbol"`
	Status        int       `json:"status"`
	SourceTime    time.Time `json:"sourceTime"`
	TargetTime    time.Time `json:"targetTime"`
}

func (r Row) OrigNet() uint32 {
	chainID, _ := strconv.Atoi(r.SourceChain)
	return uint32(chainID)
}

func (r Row) DestNet() uint32 {
	chainID, _ := strconv.Atoi(r.TargetChain)
	return uint32(chainID)
}

func (r Row) TxHash() string {
	txHash := ""
	hashs := strings.Split(r.SourceId, "-")
	if len(hashs) > 0 {
		txHash = hashs[0]
	}
	return txHash
}

func (r Row) ClaimTxHash() string {
	txHash := ""
	hashs := strings.Split(r.TargetId, "-")
	if len(hashs) > 0 {
		txHash = hashs[0]
	}
	return txHash
}

func (r Row) ReadyForClaim() bool {
	return r.Status == 99
}

func (r Row) TimeAt() uint64 {
	return uint64(r.SourceTime.Unix())
}
