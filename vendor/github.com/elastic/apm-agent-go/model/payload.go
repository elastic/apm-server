package model

// TransactionsPayload defines the payload structure expected
// by the transactions intake API.
//
// https://www.elastic.co/guide/en/apm/server/current/transaction-api.html
type TransactionsPayload struct {
	Service      *Service      `json:"service"`
	Process      *Process      `json:"process,omitempty"`
	System       *System       `json:"system,omitempty"`
	Transactions []Transaction `json:"transactions"`
}

// ErrorsPayload defines the payload structure expected
// by the errors intake API.
//
// https://www.elastic.co/guide/en/apm/server/current/error-api.html
type ErrorsPayload struct {
	Service *Service `json:"service"`
	Process *Process `json:"process,omitempty"`
	System  *System  `json:"system,omitempty"`
	Errors  []*Error `json:"errors"`
}
