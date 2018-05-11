// +build gofuzz

package model

import (
	"bytes"
	"encoding/json"

	"github.com/elastic/apm-agent-go/internal/fastjson"
	error_processor "github.com/elastic/apm-server/processor/error"
	transaction_processor "github.com/elastic/apm-server/processor/transaction"
)

func Fuzz(data []byte) int {
	type Payload struct {
		Service      *Service       `json:"service"`
		Process      *Process       `json:"process,omitempty"`
		System       *System        `json:"system,omitempty"`
		Errors       []*Error       `json:"errors"`
		Transactions []*Transaction `json:"transactions"`
	}

	var payload Payload
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return -1
	}
	raw := make(map[string]interface{})
	if err := json.Unmarshal(data, &raw); err != nil {
		return -1
	}

	if len(payload.Errors) != 0 {
		p := error_processor.NewProcessor()
		if err := p.Validate(raw); err != nil {
			return 0
		}
		payload := ErrorsPayload{
			Service: payload.Service,
			Process: payload.Process,
			System:  payload.System,
			Errors:  payload.Errors,
		}
		var w fastjson.Writer
		payload.MarshalFastJSON(&w)
		raw := make(map[string]interface{})
		if err := json.Unmarshal(w.Bytes(), &raw); err != nil {
			return -1
		}
		if err := p.Validate(raw); err != nil {
			panic(err)
		}
	}

	if len(payload.Transactions) != 0 {
		p := transaction_processor.NewProcessor()
		if err := p.Validate(raw); err != nil {
			return 0
		}
		payload := TransactionsPayload{
			Service:      payload.Service,
			Process:      payload.Process,
			System:       payload.System,
			Transactions: payload.Transactions,
		}
		var w fastjson.Writer
		payload.MarshalFastJSON(&w)
		raw := make(map[string]interface{})
		if err := json.Unmarshal(w.Bytes(), &raw); err != nil {
			return -1
		}
		if err := p.Validate(raw); err != nil {
			panic(err)
		}
	}
	return 0
}
