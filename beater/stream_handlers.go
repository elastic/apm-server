package beater

import (
	"io"
	"log"
	"net/http"

	"github.com/elastic/beats/libbeat/logp"

	conf "github.com/elastic/apm-server/config"
	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	pr "github.com/elastic/apm-server/processor"
	er "github.com/elastic/apm-server/processor/error"
	errorsSchema "github.com/elastic/apm-server/processor/error/generated-schemas"
	tr "github.com/elastic/apm-server/processor/transaction"
	transationSchema "github.com/elastic/apm-server/processor/transaction/generated-schemas"
	"github.com/pkg/errors"
)

var errorSchema = pr.CreateSchema(errorsSchema.ErrorSchema, "error")
var transactionSchema = pr.CreateSchema(transationSchema.TransactionSchema, "transaction")
var spanSchema = pr.CreateSchema(transationSchema.SpanSchema, "transaction")

var (
	errNoTypeProperty = errors.New("missing 'type' property")
	errUnknownType    = errors.New("unknown 'type' supplied")
)

// var log = logp.NewLogger("streamdecoder")

func validate(objType string, value interface{}) error {
	switch objType {
	case "header":
		// TODO: validate header
		return nil
	case "error":
		return errorSchema.ValidateInterface(value)
	case "transaction":
		return transactionSchema.ValidateInterface(value)
	case "span":
		return spanSchema.ValidateInterface(value)
	default:
		log.Printf("unknown type %s\n", objType)
		return errUnknownType
	}
}

func decode(objType string, value interface{}) (model.Transformable, error) {
	var err error
	switch objType {
	case "error":
		return er.DecodeEvent(value, err)
	case "transaction":
		return tr.DecodeTransaction(value, err)
	case "span":
		return tr.DecodeSpan(value, err)
	default:
		return nil, errUnknownType
	}
}

func batchedStreamProcessing(r *http.Request, entityReader decoder.EntityStreamReader, batchSize int) ([]model.Transformable, error) {
	batch := make([]model.Transformable, 0, batchSize)
	for {
		item, err := getEntityFromStream(entityReader)
		if err != nil {
			return batch, err
		}

		batch = append(batch, item)

		if len(batch) >= batchSize {
			return batch, nil
		}
	}
}

func extractEntityType(rawEntity map[string]interface{}) (string, error) {
	objType, ok := rawEntity["_type"].(string)
	if !ok {
		return "", errNoTypeProperty
	}
	return objType, nil
}

func getEntityFromStream(entityReader decoder.EntityStreamReader) (model.Transformable, error) {
	sr, err := entityReader()
	if err != nil {
		return nil, err
	}

	objType, err := extractEntityType(sr)
	if err != nil {
		return nil, err
	}

	err = validate(objType, sr)
	if err != nil {
		return nil, err
	}

	return decode(objType, sr)
}

func processStreamRequest(transformBatchSize int, config conf.TransformConfig, report reporter, extractors []decoder.Extractor, streamDecoder decoder.StreamDecoder) http.Handler {
	sthandler := func(r *http.Request) serverResponse {
		reader, err := streamDecoder(r)
		if err != nil {
			return serverResponse{errors.Wrap(err, "while decoding"), 400, nil}
		}

		rawHeader, err := reader()
		if err != nil {
			return serverResponse{errors.Wrap(err, "while decoding"), 400, nil}
		}

		htype, err := extractEntityType(rawHeader)
		if err != nil || htype != "header" {
			return cannotValidateResponse(errors.New("missing header"))
		}

		agumenter := decoder.GetAugmenter(r, extractors)
		agumenter.Augment(rawHeader)

		transformContext, err := model.DecodeContext(rawHeader, err)

		for {
			batch, err := batchedStreamProcessing(r, reader, v2TransformBatchSize)

			if err != nil && err != io.EOF {
				return serverResponse{err, http.StatusInternalServerError, nil}
			}

			report(pendingReq{
				batch, config, transformContext,
			})
			logp.Info("reported back on")

			if err == io.EOF {
				return okResponse
			}
		}

		return okResponse
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		sendStatus(w, r, sthandler(r))
	}

	return http.HandlerFunc(handler)
}
