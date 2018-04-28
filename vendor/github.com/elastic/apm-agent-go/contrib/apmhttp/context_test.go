package apmhttp_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-agent-go/contrib/apmhttp"
)

func TestStatusCode(t *testing.T) {
	for i := 100; i < 600; i++ {
		assert.Equal(t, strconv.Itoa(i), apmhttp.StatusCodeString(i))
	}
}
