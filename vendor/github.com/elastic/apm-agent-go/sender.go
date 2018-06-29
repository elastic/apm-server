package elasticapm

import (
	"context"

	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/stacktrace"
)

type sender struct {
	tracer *Tracer
	cfg    *tracerConfig
	stats  *TracerStats

	modelTransactions []model.Transaction
	modelSpans        []model.Span
	modelStacktrace   []model.StacktraceFrame
}

// sendTransactions attempts to send enqueued transactions to the APM server,
// returning true if the transactions were successfully sent.
func (s *sender) sendTransactions(ctx context.Context, transactions []*Transaction) bool {
	if len(transactions) == 0 {
		return false
	}

	s.modelTransactions = s.modelTransactions[:0]
	s.modelSpans = s.modelSpans[:0]
	s.modelStacktrace = s.modelStacktrace[:0]
	var spanOffset int
	var stacktraceOffset int

	for _, tx := range transactions {
		s.modelTransactions = append(s.modelTransactions, model.Transaction{
			Name:      truncateString(tx.Name),
			Type:      truncateString(tx.Type),
			ID:        tx.id,
			Result:    truncateString(tx.Result),
			Timestamp: model.Time(tx.Timestamp.UTC()),
			Duration:  tx.Duration.Seconds() * 1000,
			SpanCount: model.SpanCount{
				Dropped: model.SpanCountDropped{
					Total: tx.spansDropped,
				},
			},
		})
		modelTx := &s.modelTransactions[len(s.modelTransactions)-1]
		if tx.Sampled() {
			modelTx.Context = tx.Context.build()
			if s.cfg.sanitizedFieldNames != nil && modelTx.Context != nil && modelTx.Context.Request != nil {
				sanitizeRequest(modelTx.Context.Request, s.cfg.sanitizedFieldNames)
			}
			for _, span := range tx.spans {
				s.modelSpans = append(s.modelSpans, model.Span{
					ID:       &span.id,
					Name:     truncateString(span.Name),
					Type:     truncateString(span.Type),
					Start:    span.Timestamp.Sub(tx.Timestamp).Seconds() * 1000,
					Duration: span.Duration.Seconds() * 1000,
					Context:  span.Context.build(),
				})
				modelSpan := &s.modelSpans[len(s.modelSpans)-1]
				if span.parent != -1 {
					modelSpan.Parent = &span.parent
				}
				s.modelStacktrace = appendModelStacktraceFrames(s.modelStacktrace, span.stacktrace)
				modelSpan.Stacktrace = s.modelStacktrace[stacktraceOffset:]
				stacktraceOffset += len(span.stacktrace)
				s.setStacktraceContext(modelSpan.Stacktrace)
			}
			modelTx.Spans = s.modelSpans[spanOffset:]
			spanOffset += len(tx.spans)
		} else {
			modelTx.Sampled = &tx.sampled
		}
	}

	service := makeService(s.tracer.Service.Name, s.tracer.Service.Version, s.tracer.Service.Environment)
	payload := model.TransactionsPayload{
		Service:      &service,
		Process:      s.tracer.process,
		System:       s.tracer.system,
		Transactions: s.modelTransactions,
	}

	if err := s.tracer.Transport.SendTransactions(ctx, &payload); err != nil {
		if s.cfg.logger != nil {
			s.cfg.logger.Debugf("sending transactions failed: %s", err)
		}
		s.stats.Errors.SendTransactions++
		return false
	}
	s.stats.TransactionsSent += uint64(len(transactions))
	return true
}

// sendErrors attempts to send enqueued errors to the APM server,
// returning true if the errors were successfully sent.
func (s *sender) sendErrors(ctx context.Context, errors []*Error) bool {
	if len(errors) == 0 {
		return false
	}
	service := makeService(s.tracer.Service.Name, s.tracer.Service.Version, s.tracer.Service.Environment)
	payload := model.ErrorsPayload{
		Service: &service,
		Process: s.tracer.process,
		System:  s.tracer.system,
		Errors:  make([]*model.Error, len(errors)),
	}
	for i, e := range errors {
		if e.Transaction != nil {
			e.model.Transaction.ID = e.Transaction.id
		}
		s.setStacktraceContext(e.modelStacktrace)
		e.setStacktrace()
		e.setCulprit()
		e.model.ID = e.ID
		e.model.Timestamp = model.Time(e.Timestamp.UTC())
		e.model.Context = e.Context.build()
		e.model.Exception.Handled = e.Handled
		payload.Errors[i] = &e.model
	}
	if err := s.tracer.Transport.SendErrors(ctx, &payload); err != nil {
		if s.cfg.logger != nil {
			s.cfg.logger.Debugf("sending errors failed: %s", err)
		}
		s.stats.Errors.SendErrors++
		return false
	}
	s.stats.ErrorsSent += uint64(len(errors))
	return true
}

func (s *sender) setStacktraceContext(stack []model.StacktraceFrame) {
	if s.cfg.contextSetter == nil || len(stack) == 0 {
		return
	}
	err := stacktrace.SetContext(s.cfg.contextSetter, stack, s.cfg.preContext, s.cfg.postContext)
	if s.cfg.logger != nil {
		s.cfg.logger.Debugf("setting context failed: %s", err)
	}
	s.stats.Errors.SetContext++
}
