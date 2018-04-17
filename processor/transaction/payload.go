package transaction

import (
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	transformations    = monitoring.NewInt(transactionMetrics, "transformations")
	transactionCounter = monitoring.NewInt(transactionMetrics, "transactions")
	spanCounter        = monitoring.NewInt(transactionMetrics, "spans")
	stacktraceCounter  = monitoring.NewInt(transactionMetrics, "stacktraces")
	frameCounter       = monitoring.NewInt(transactionMetrics, "frames")

	processorTransEntry = common.MapStr{"name": processorName, "event": transactionDocType}
	processorSpanEntry  = common.MapStr{"name": processorName, "event": spanDocType}
)

type Payload struct {
	Service m.Service
	System  *m.System
	Process *m.Process
	User    *m.User
	Events  []*Transaction
}

func DecodePayload(raw map[string]interface{}) ([]*Transaction, error) {
	if raw == nil {
		return nil, nil
	}

	var err error
	decoder := utility.ManualDecoder{}
	txs := decoder.InterfaceArr(raw, "transactions")
	err = decoder.Err
	transactions := make([]*Transaction, len(txs))
	for idx, tx := range txs {
		transactions[idx], err = DecodeTransaction(tx, err)
	}
	return transactions, err
}
