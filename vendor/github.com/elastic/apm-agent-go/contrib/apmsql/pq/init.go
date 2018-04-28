package apmpq

import (
	"github.com/lib/pq"

	"github.com/elastic/apm-agent-go/contrib/apmsql"
)

func init() {
	apmsql.Register("postgres", &pq.Driver{})
}
