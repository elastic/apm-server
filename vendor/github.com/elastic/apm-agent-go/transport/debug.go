package transport

import (
	"context"
	"log"
	"sync/atomic"

	"github.com/elastic/apm-agent-go/internal/pretty"
	"github.com/elastic/apm-agent-go/model"
)

type debugTransport struct {
	id        uint64
	transport Transport
}

func (dt *debugTransport) SendTransactions(ctx context.Context, p *model.TransactionsPayload) error {
	id := atomic.AddUint64(&dt.id, 1)
	log.Printf("elasticapm SendTransactions %d -> %# v", id, pretty.Formatter(p))
	err := dt.transport.SendTransactions(ctx, p)
	log.Printf("elasticapm SendTransactions %d <- %v", id, err)
	return err
}

func (dt *debugTransport) SendErrors(ctx context.Context, p *model.ErrorsPayload) error {
	id := atomic.AddUint64(&dt.id, 1)
	log.Printf("elasticapm SendErrors %d -> %# v", id, pretty.Formatter(p))
	err := dt.transport.SendErrors(ctx, p)
	log.Printf("elasticapm SendErrors %d <- %v", id, err)
	return err
}
