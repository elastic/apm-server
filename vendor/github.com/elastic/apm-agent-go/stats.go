package elasticapm

// TracerStats holds statistics for a Tracer.
type TracerStats struct {
	Errors              TracerStatsErrors
	ErrorsSent          uint64
	ErrorsDropped       uint64
	TransactionsSent    uint64
	TransactionsDropped uint64
}

// TracerStatsErrors holds error statistics for a Tracer.
type TracerStatsErrors struct {
	SetContext       uint64
	SendTransactions uint64
	SendErrors       uint64
}

func (s TracerStats) isZero() bool {
	return s == TracerStats{}
}

// accumulate updates the stats by accumulating them with
// the values in rhs.
func (s *TracerStats) accumulate(rhs TracerStats) {
	s.Errors.SetContext += rhs.Errors.SetContext
	s.Errors.SendTransactions += rhs.Errors.SendTransactions
	s.Errors.SendErrors += rhs.Errors.SendErrors
	s.ErrorsSent += rhs.ErrorsSent
	s.ErrorsDropped += rhs.ErrorsDropped
	s.TransactionsSent += rhs.TransactionsSent
	s.TransactionsDropped += rhs.TransactionsDropped
}
