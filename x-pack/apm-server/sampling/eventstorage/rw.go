package eventstorage

import (
	"github.com/elastic/apm-data/model/modelpb"
)

type RW interface {
	ReadTraceEvents(traceID string, out *modelpb.Batch) error
	WriteTraceEvent(traceID, id string, event *modelpb.APMEvent, opts WriterOpts) error
	WriteTraceSampled(traceID string, sampled bool, opts WriterOpts) error
	IsTraceSampled(traceID string) (bool, error)
	DeleteTraceEvent(traceID, id string) error
	Flush() error
}
