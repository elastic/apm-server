package apm

// TracerStats holds statistics for a Tracer.
type TracerStats struct {
	Errors              TracerStatsErrors
	ErrorsSent          uint64
	ErrorsDropped       uint64
	TransactionsSent    uint64
	TransactionsDropped uint64
	SpansSent           uint64
	SpansDropped        uint64
}

// TracerStatsErrors holds error statistics for a Tracer.
type TracerStatsErrors struct {
	SetContext uint64
	SendStream uint64
}

func (s TracerStats) isZero() bool {
	return s == TracerStats{}
}

// accumulate updates the stats by accumulating them with
// the values in rhs.
func (s *TracerStats) accumulate(rhs TracerStats) {
	s.Errors.SetContext += rhs.Errors.SetContext
	s.Errors.SendStream += rhs.Errors.SendStream
	s.ErrorsSent += rhs.ErrorsSent
	s.ErrorsDropped += rhs.ErrorsDropped
	s.SpansSent += rhs.SpansSent
	s.SpansDropped += rhs.SpansDropped
	s.TransactionsSent += rhs.TransactionsSent
	s.TransactionsDropped += rhs.TransactionsDropped
}
