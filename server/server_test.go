package server

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/tests"
)

func TestDecode(t *testing.T) {
	transactionBytes, err := tests.LoadValidData("transaction")
	assert.Nil(t, err)
	buffer := bytes.NewReader(transactionBytes)
	req, err := http.NewRequest("POST", "/transactions", buffer)
	req.Header.Add("Content-Type", "application/json")
	res, err := decodeData(req)
	assert.Nil(t, err)
	assert.NotNil(t, res)
}
