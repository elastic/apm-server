package transaction

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/apm-server/config"
	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Span struct {
	Name       string
	Type       string
	Start      float64
	Duration   float64
	Timestamp  time.Time
	Context    common.MapStr
	Stacktrace m.Stacktrace

	// new interface
	HexId         *string
	ParentId      *string
	TraceId       *string
	TransactionId *string

	// old interface
	Id     *int64
	Parent *int64
}

func DecodeSpan(input interface{}, err error) (*Span, error) {
	sp, err := decodeCommonSpan(input, err)
	if err != nil {
		return nil, err
	}
	// typecast already checked in `decodeCommonSpan`
	raw, _ := input.(map[string]interface{})

	decoder := utility.ManualDecoder{}
	sp.Id = decoder.Int64Ptr(raw, "id")
	sp.Parent = decoder.Int64Ptr(raw, "parent")
	if decoder.Err != nil {
		return nil, decoder.Err
	}
	return sp, nil
}

func DecodeDtSpan(input interface{}, err error) (*Span, error) {
	sp, err := decodeCommonSpan(input, err)
	if err != nil {
		return nil, err
	}
	// typecast already checked in `decodeCommonSpan`
	raw, _ := input.(map[string]interface{})

	decoder := utility.ManualDecoder{}
	sp.ParentId = decoder.StringPtr(raw, "parent_id")
	hexId := decoder.String(raw, "id")
	traceId := decoder.String(raw, "trace_id")
	transactionId := decoder.String(raw, "transaction_id")

	if decoder.Err != nil {
		return nil, decoder.Err
	}
	sp.HexId = &hexId
	sp.TraceId = &traceId

	// Ensure backwards compatibility:

	// HexId must be a 64 bit hex encoded string. The id is set to the integer
	// converted value of the hexId
	if idInt, err := HexToInt(*sp.HexId, 64); err == nil {
		sp.Id = &idInt
	} else {
		return nil, err
	}

	// - set parent to parentId if it doesn't point to the transaction
	if sp.ParentId != nil && sp.TransactionId != sp.ParentId {
		if id, err := HexToInt(*sp.ParentId, 64); err == nil {
			sp.Parent = &id
		} else {
			return sp, err
		}
	}

	// - set transactionId to `traceId:transactionId`
	//   for global uniqueness when queried from old UI
	trId := strings.Join([]string{traceId, transactionId}, "-")
	sp.TransactionId = &trId

	return sp, nil
}

func (s *Span) Transform(config config.Config, service m.Service) common.MapStr {
	if s == nil {
		return nil
	}
	tr := common.MapStr{}
	utility.Add(tr, "id", s.Id)
	utility.Add(tr, "hex_id", s.HexId)
	utility.Add(tr, "trace_id", s.TraceId)
	utility.Add(tr, "parent_id", s.ParentId)
	utility.Add(tr, "parent", s.Parent)
	utility.Add(tr, "name", s.Name)
	utility.Add(tr, "type", s.Type)
	utility.Add(tr, "start", utility.MillisAsMicros(s.Start))
	utility.Add(tr, "duration", utility.MillisAsMicros(s.Duration))
	st := s.Stacktrace.Transform(config, service)
	utility.Add(tr, "stacktrace", st)
	return tr
}

func decodeCommonSpan(input interface{}, err error) (*Span, error) {
	if err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for span")
	}
	decoder := utility.ManualDecoder{}
	sp := Span{
		Name:      decoder.String(raw, "name"),
		Type:      decoder.String(raw, "type"),
		Start:     decoder.Float64(raw, "start"),
		Duration:  decoder.Float64(raw, "duration"),
		Context:   decoder.MapStr(raw, "context"),
		Timestamp: decoder.TimeRFC3339WithDefault(raw, "timestamp"),
	}

	var stacktr *m.Stacktrace
	stacktr, err = m.DecodeStacktrace(raw["stacktrace"], decoder.Err)
	if stacktr != nil {
		sp.Stacktrace = *stacktr
	}
	return &sp, err
}

var shift = uint64(math.Pow(2, 63))

func HexToInt(s string, bitSize int) (int64, error) {
	us, err := strconv.ParseUint(s, 16, bitSize)
	if err != nil {
		return 0, err
	}
	return int64(us - shift), nil
}
