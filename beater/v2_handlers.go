package beater

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/elastic/apm-server/utility"

	"github.com/elastic/apm-server/transform"
	"github.com/pkg/errors"

	"github.com/elastic/apm-server/validation"
	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/decoder"
	er "github.com/elastic/apm-server/model/error"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/metric"
	"github.com/elastic/apm-server/model/span"
	"github.com/elastic/apm-server/model/transaction"
)

type streamResponse struct {
	Errors   map[int]map[string]uint `json:"errors"`
	Accepted uint                    `json:"accepted"`
	Invalid  uint                    `json:"invalid"`
	Dropped  uint                    `json:"dropped"`
}

func (s *streamResponse) addErrorCount(serverResponse serverResponse, count int) {
	if s.Errors == nil {
		s.Errors = make(map[int]map[string]uint)
	}

	errorMsgs, ok := s.Errors[serverResponse.code]
	if !ok {
		s.Errors[serverResponse.code] = make(map[string]uint)
		errorMsgs = s.Errors[serverResponse.code]
	}
	errorMsgs[serverResponse.err.Error()] += uint(count)
}

func (s *streamResponse) addError(serverResponse serverResponse) {
	s.addErrorCount(serverResponse, 1)
}

type NDJSONStreamReader struct {
	stream *bufio.Reader
	isEOF  bool
}

const batchSize = 10

func (sr *NDJSONStreamReader) Read() (map[string]interface{}, error) {
	// ReadBytes can return valid data in `buf` _and_ also an io.EOF
	buf, readErr := sr.stream.ReadBytes('\n')
	if readErr != nil && readErr != io.EOF {
		return nil, readErr
	}

	sr.isEOF = readErr == io.EOF

	if len(buf) == 0 {
		return nil, readErr
	}

	tmpreader := ioutil.NopCloser(bytes.NewBuffer(buf))
	decoded, err := decoder.DecodeJSONData(tmpreader)
	if err != nil {
		return nil, err
	}

	return decoded, readErr // this might be io.EOF
}

func (n *NDJSONStreamReader) SkipToEnd() (uint, error) {
	objects := uint(0)
	nl := []byte("\n")
	var readErr error
	for readErr == nil {
		countBuf := make([]byte, 2048)
		_, readErr = n.stream.Read(countBuf)
		objects += uint(bytes.Count(countBuf, nl))
	}
	return objects, readErr
}

func StreamDecodeLimitJSONData(req *http.Request, maxSize int64) (*NDJSONStreamReader, error) {
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/x-ndjson") {
		return nil, fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
	}

	reader, err := decoder.CompressedRequestReader(maxSize)(req)
	if err != nil {
		return nil, err
	}

	return &NDJSONStreamReader{bufio.NewReader(reader), false}, nil
}

type v2Route struct {
	wrappingHandler     func(h http.Handler) http.Handler
	configurableDecoder func(*Config, decoder.ReqDecoder) decoder.ReqDecoder
	transformConfig     func(*Config) transform.Config
}

func (v v2Route) Handler(beaterConfig *Config, report reporter) http.Handler {
	reqDecoder := v.configurableDecoder(
		beaterConfig,
		func(*http.Request) (map[string]interface{}, error) { return map[string]interface{}{}, nil },
	)

	v2Handler := v2Handler{
		requestDecoder: reqDecoder,
		tconfig:        v.transformConfig(beaterConfig),
	}

	return v.wrappingHandler(v2Handler.Handle(beaterConfig, report))
}

var Models = []struct {
	key          string
	schema       *jsonschema.Schema
	modelDecoder func(interface{}, error) (transform.Transformable, error)
}{
	{
		"transaction",
		transaction.ModelSchema(),
		transaction.DecodeEvent,
	},
	{
		"span",
		span.ModelSchema(),
		span.DecodeSpan,
	},
	{
		"metric",
		metric.ModelSchema(),
		metric.DecodeMetric,
	},
	{
		"error",
		er.ModelSchema(),
		er.DecodeEvent,
	},
}

type v2Handler struct {
	requestDecoder decoder.ReqDecoder
	tconfig        transform.Config
}

// handleRawModel validates and decodes a single json object into its struct form
func (v *v2Handler) handleRawModel(rawModel map[string]interface{}) (transform.Transformable, serverResponse) {
	for _, model := range Models {
		if entry, ok := rawModel[model.key]; ok {
			err := validation.Validate(entry, model.schema)
			if err != nil {
				return nil, cannotValidateResponse(err)
			}

			tr, err := model.modelDecoder(entry, err)
			if err != nil {
				return tr, cannotDecodeResponse(err)
			}
			return tr, serverResponse{}
		}
	}
	return nil, cannotValidateResponse(errors.New("did not recognize object type"))
}

// readBatch will read up to `batchSize` objects from the ndjson stream
// it returns a slice of eventables, a serverResponse and a bool that indicates if we're at EOF.
func (v *v2Handler) readBatch(batchSize int, reader *NDJSONStreamReader, response *streamResponse) ([]transform.Transformable, bool) {
	var err error
	var rawModel map[string]interface{}

	eventables := []transform.Transformable{}
	for i := 0; i < batchSize && err == nil; i++ {
		rawModel, err = reader.Read()
		if err != nil && err != io.EOF {
			response.addError(cannotDecodeResponse(err))
			response.Invalid++
		}

		if rawModel != nil {
			tr, resp := v.handleRawModel(rawModel)
			if resp.IsError() {
				response.addError(resp)
				response.Invalid++
			}
			eventables = append(eventables, tr)
		}
	}

	return eventables, reader.isEOF
}
func (v *v2Handler) readMetadata(r *http.Request, ndjsonReader *NDJSONStreamReader) (*metadata.Metadata, serverResponse) {
	// first item is the metadata object
	rawData, err := ndjsonReader.Read()
	if err != nil {
		return nil, cannotDecodeResponse(err)
	}

	rawMetadata, ok := rawData["metadata"].(map[string]interface{})
	if !ok {
		return nil, cannotValidateResponse(errors.New("invalid metadata format"))
	}

	// augment the metadata object with information from the request, like user-agent or remote address
	reqMeta, err := v.requestDecoder(r)
	if err != nil {
		return nil, cannotDecodeResponse(err)
	}

	for k, v := range reqMeta {
		utility.MergeAdd(rawMetadata, k, v.(map[string]interface{}))
	}

	// validate the metadata object against our jsonschema
	err = validation.Validate(rawMetadata, metadata.ModelSchema())
	if err != nil {
		return nil, cannotValidateResponse(err)
	}

	// create a metadata struct
	metadata, err := metadata.DecodeMetadata(rawMetadata)
	if err != nil {
		return nil, cannotDecodeResponse(err)
	}
	return metadata, serverResponse{}
}

func (v *v2Handler) handleRequest(r *http.Request, ndjsonReader *NDJSONStreamReader, report reporter) *streamResponse {
	resp := &streamResponse{}

	metadata, serverResponse := v.readMetadata(r, ndjsonReader)
	log.Println("READ METADATA")
	// no point in continueing if we couldn't read the metadata
	if serverResponse.IsError() {
		log.Println("METADATA err")
		sr := streamResponse{}
		sr.addError(serverResponse)
		return &sr
	}

	tctx := &transform.Context{
		Config:   v.tconfig,
		Metadata: *metadata,
	}

	for {
		transformables, eof := v.readBatch(batchSize, ndjsonReader, resp)
		log.Println("READ BATCH")
		if transformables != nil {
			err := report(r.Context(), pendingReq{
				transformables: transformables,
				tcontext:       tctx,
			})
			if err != nil {
				if strings.Contains(err.Error(), "publisher is being stopped") {
					sr := streamResponse{}
					sr.addError(serverShuttingDownResponse(err))
					return &sr
				}

				resp.addErrorCount(fullQueueResponse(err), len(transformables))
				resp.Dropped += uint(len(transformables))
			}
		}

		if eof {
			break
		}
	}
	return resp
}

func (v *v2Handler) sendResponse(w http.ResponseWriter, streamResponse *streamResponse) error {
	statusCode := http.StatusAccepted
	for k := range streamResponse.Errors {
		if k > statusCode {
			statusCode = k
		}
	}
	log.Println(statusCode, streamResponse)
	w.WriteHeader(statusCode)
	if statusCode != http.StatusAccepted {
		return json.NewEncoder(w).Encode(streamResponse)
	}
	return nil
}

func (v *v2Handler) Handle(beaterConfig *Config, report reporter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("BLEH")
		ndjsonReader, err := StreamDecodeLimitJSONData(r, beaterConfig.MaxUnzippedSize)
		if err != nil {
			sr := streamResponse{}
			sr.addError(cannotDecodeResponse(err))
		}
		log.Println("BLEH2")
		streamResponse := v.handleRequest(r, ndjsonReader, report)
		log.Println("BLEH3")
		// did we return early?
		if !ndjsonReader.isEOF {
			log.Println("DID NOT RETURN EARLY")

			dropped, err := ndjsonReader.SkipToEnd()
			if err != io.EOF {
				// trouble
				log.Println("TROUBLE1")
			}
			streamResponse.Dropped += dropped
		}

		if err := v.sendResponse(w, streamResponse); err != nil {
			log.Println("TROUBLE2", err)
			// trouble
		}
	})
}

// var V2Routes = map[string]v2Route{
// 	V2BackendURL:  v2Route{BackendRouteType},
// 	V2FrontendURL: v2Route{FrontendRouteType},
// }
