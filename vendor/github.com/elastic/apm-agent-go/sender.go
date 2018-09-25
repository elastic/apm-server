package elasticapm

import (
	"context"
	"sync"
	"time"

	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/stacktrace"
)

// notSampled is used as the pointee for the model.Transaction.Sampled field
// of non-sampled transactions.
var notSampled = false

type sender struct {
	tracer  *Tracer
	cfg     *tracerConfig
	stats   *TracerStats
	metrics Metrics

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

	for _, tx := range transactions {
		s.modelTransactions = append(s.modelTransactions, model.Transaction{})
		modelTx := &s.modelTransactions[len(s.modelTransactions)-1]
		s.buildModelTransaction(modelTx, tx)
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
			if !e.Transaction.traceContext.Span.isZero() {
				e.model.TraceID = model.TraceID(e.Transaction.traceContext.Trace)
				e.model.ParentID = model.SpanID(e.Transaction.traceContext.Span)
			} else {
				e.model.Transaction.ID = model.UUID(e.Transaction.traceContext.Trace)
			}
		}
		s.setStacktraceContext(e.modelStacktrace)
		e.setStacktrace()
		e.setCulprit()
		e.model.ID = model.UUID(e.ID)
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

// gatherMetrics gathers metrics from each of the registered
// metrics gatherers. Once all gatherers have returned, a value
// will be sent on the "gathered" channel.
func (s *sender) gatherMetrics(ctx context.Context, gathered chan<- struct{}) {
	// s.cfg must not be used within the goroutines, as it may be
	// concurrently mutated by the main tracer goroutine. Take a
	// copy of the current config.
	logger := s.cfg.logger

	timestamp := model.Time(time.Now().UTC())
	var group sync.WaitGroup
	for _, g := range s.cfg.metricsGatherers {
		group.Add(1)
		go func(g MetricsGatherer) {
			defer group.Done()
			gatherMetrics(ctx, g, &s.metrics, logger)
		}(g)
	}

	go func() {
		group.Wait()
		for _, m := range s.metrics.metrics {
			m.Timestamp = timestamp
		}
		gathered <- struct{}{}
	}()
}

// sendMetrics attempts to send metrics to the APM server. This must be
// called after gatherMetrics has signalled that metrics have all been
// gathered.
func (s *sender) sendMetrics(ctx context.Context) {
	if len(s.metrics.metrics) == 0 {
		return
	}
	service := makeService(s.tracer.Service.Name, s.tracer.Service.Version, s.tracer.Service.Environment)
	payload := model.MetricsPayload{
		Service: &service,
		Process: s.tracer.process,
		System:  s.tracer.system,
		Metrics: s.metrics.metrics,
	}
	if err := s.tracer.Transport.SendMetrics(ctx, &payload); err != nil {
		if s.cfg.logger != nil {
			s.cfg.logger.Debugf("sending metrics failed: %s", err)
		}
	}
	s.metrics.reset()
}

func (s *sender) buildModelTransaction(out *model.Transaction, tx *Transaction) {
	if !tx.traceContext.Span.isZero() {
		out.TraceID = model.TraceID(tx.traceContext.Trace)
		out.ParentID = model.SpanID(tx.parentSpan)
		out.ID.SpanID = model.SpanID(tx.traceContext.Span)
	} else {
		out.ID.UUID = model.UUID(tx.traceContext.Trace)
	}

	out.Name = truncateString(tx.Name)
	out.Type = truncateString(tx.Type)
	out.Result = truncateString(tx.Result)
	out.Timestamp = model.Time(tx.Timestamp.UTC())
	out.Duration = tx.Duration.Seconds() * 1000
	out.SpanCount.Dropped.Total = tx.spansDropped

	if !tx.Sampled() {
		out.Sampled = &notSampled
	}

	out.Context = tx.Context.build()
	if s.cfg.sanitizedFieldNames != nil && out.Context != nil && out.Context.Request != nil {
		sanitizeRequest(out.Context.Request, s.cfg.sanitizedFieldNames)
	}

	spanOffset := len(s.modelSpans)
	for _, span := range tx.spans {
		s.modelSpans = append(s.modelSpans, model.Span{})
		modelSpan := &s.modelSpans[len(s.modelSpans)-1]
		s.buildModelSpan(modelSpan, span)
	}
	out.Spans = s.modelSpans[spanOffset:]
}

func (s *sender) buildModelSpan(out *model.Span, span *Span) {
	if !span.tx.traceContext.Span.isZero() {
		out.ID = model.SpanID(span.id)
		out.ParentID = model.SpanID(span.parent)
		out.TraceID = model.TraceID(span.tx.traceContext.Trace)
	}

	out.Name = truncateString(span.Name)
	out.Type = truncateString(span.Type)
	out.Start = span.Timestamp.Sub(span.tx.Timestamp).Seconds() * 1000
	out.Duration = span.Duration.Seconds() * 1000
	out.Context = span.Context.build()

	stacktraceOffset := len(s.modelStacktrace)
	s.modelStacktrace = appendModelStacktraceFrames(s.modelStacktrace, span.stacktrace)
	out.Stacktrace = s.modelStacktrace[stacktraceOffset:]
	s.setStacktraceContext(out.Stacktrace)
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
