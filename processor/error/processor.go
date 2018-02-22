package error

import (
	"github.com/santhosh-tekuri/jsonschema"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/monitoring"
	"github.com/elastic/apm-server/model"
	"time"
)

var (
	errorMetrics    = monitoring.Default.NewRegistry("apm-server.processor.error")
	validationCount = monitoring.NewInt(errorMetrics, "validation.count")
	validationError = monitoring.NewInt(errorMetrics, "validation.errors")
	transformations = monitoring.NewInt(errorMetrics, "transformations")
)

const (
	processorName = "error"
	errorDocType  = "error"
)

var schema = pr.CreateSchema(errorSchema, processorName)

func NewProcessor(config *pr.Config) pr.Processor {
	return &processor{schema: schema, config: config}
}

type processor struct {
	schema *jsonschema.Schema
	config *pr.Config
}

func (p *processor) Validate(raw map[string]interface{}) error {
	validationCount.Inc()
	err := pr.Validate(raw, p.schema)
	if err != nil {
		validationError.Inc()
	}
	return err
}

func (p *processor) Transform(raw interface{}) ([]beat.Event, error) {
	transformations.Inc()
	return decode(raw.(map[string]interface{})).transform(p.config), nil
}

func (p *processor) Name() string {
	return processorName
}

func decode(raw map[string]interface{}) *payload {

	pa := payload{Process:&model.Process{}, System:&model.System{}}
	pa.Name, _ = utility.DeepGet(raw, "service")["name"].(string)
	serviceVersion, _ := utility.DeepGet(raw, "service")["version"].(string)
	pa.Version = &serviceVersion
	pa.AgentName, _ = utility.DeepGet(raw, "service", "agent")["name"].(string)
	pa.AgentVersion, _ = utility.DeepGet(raw, "service", "agent")["version"].(string)
	frameworkName, _ := utility.DeepGet(raw, "service", "framework")["name"].(string)
	pa.FrameworkName = &frameworkName
	frameworkVersion, _ := utility.DeepGet(raw, "service", "framework")["version"].(string)
	pa.FrameworkVersion = &frameworkVersion
	runtimeName, _ := utility.DeepGet(raw, "service", "runtime")["name"].(string)
	pa.RuntimeName = &runtimeName
	runtimeVersion, _ := utility.DeepGet(raw, "service", "runtime")["version"].(string)
	pa.RuntimeVersion = &runtimeVersion
	languageName, _ := utility.DeepGet(raw, "service", "language")["name"].(string)
	pa.LanguageName = &languageName
	languageVersion, _ := utility.DeepGet(raw, "service", "language")["version"].(string)
	pa.LanguageVersion = &languageVersion
	environment, _ := utility.DeepGet(raw, "service")["environment"].(string)
	pa.Environment = &environment

	pa.Pid, _ = utility.DeepGet(raw, "process")["pid"].(int)
	ppid, _ := utility.DeepGet(raw, "process")["ppid"].(int)
	pa.Ppid = &ppid
	title, _ := utility.DeepGet(raw, "process")["title"].(string)
	pa.Title = &title
	pa.Argv, _ = utility.DeepGet(raw, "process")["argv"].([]string)

	hostname, _ := utility.DeepGet(raw, "system")["hostname"].(string)
	pa.Hostname = &hostname
	platform, _ := utility.DeepGet(raw, "system")["platform"].(string)
	pa.Platform = &platform
	architecture, _ := utility.DeepGet(raw, "system")["architecture"].(string)
	pa.Architecture = &architecture

	if errs, ok := raw["errors"].([]interface{}); ok {
		pa.Events = make([]Event, len(errs))
		for errIdx, err := range errs {
			err, ok := err.(map[string]interface{})
			if !ok {
				continue
			}
			event := Event{Log:&Log{}, Exception:&Exception{}, Transaction:&Transaction{}}
			event.Context = utility.DeepGet(err, "context")
			id, _ := err["id"].(string)
			event.Id = &id
			if timestamp, ok := err["timestamp"].(string); ok {
				event.Timestamp, _ = time.Parse(time.RFC3339, timestamp)
			}
			culprit, _ := err["culprit"].(string)
			event.Culprit = &culprit

			log := utility.DeepGet(err, "log")
			event.LogMessage, _ = log["message"].(string)
			paramMessage, _ := log["param_message"].(string)
			event.LogParamMessage = &paramMessage
			loggerName, _ := log["logger_name"].(string)
			event.LoggerName = &loggerName
			logLevel, _ := log["level"].(string)
			event.LogLevel = &logLevel

			if frames, ok := log["stacktrace"].([]interface{}); ok {
				event.LogStacktrace = make(model.Stacktrace, len(frames))
				for frIdx, fr := range frames {
					fr, ok := fr.(map[string]interface{})
					if !ok {
						continue
					}
					frame := model.StacktraceFrame{}
					function, _  := fr["function"].(string)
					frame.Function = &function
					absPath, _ := fr["abs_path"].(string)
					frame.AbsPath = &absPath
					frame.Filename, _ = fr["filename"].(string)
					frame.Lineno, _ = fr["lineno"].(int)
					libraryFrame, _ := fr["library_frame"].(bool)
					frame.LibraryFrame = &libraryFrame
					frame.Vars = utility.DeepGet(fr, "vars")
					module, _ := fr["module"].(string)
					frame.Module = &module
					colno, _ := fr["colno"].(int)
					frame.Colno = &colno
					contextLine, _ := fr["context_line"].(string)
					frame.ContextLine = &contextLine
					frame.PreContext, _ = fr["pre_context"].([]string)
					frame.PostContext, _ = fr["post_context"].([]string)
					event.LogStacktrace[frIdx] = &frame
				}
			} else {
				event.LogStacktrace = make(model.Stacktrace, 0)
			}

			event.TransactionId, _ = utility.DeepGet(err,"transaction")["id"].(string)
			ex := utility.DeepGet(err, "exception")
			event.ExMessage, _ = ex["message"].(string)
			exType, _ := ex["type"].(string)
			event.ExType = &exType
			exModule, _ := ex["module"].(string)
			event.ExModule = &exModule
			event.ExCode = ex["code"]
			exHandled, _ := ex["handled"].(bool)
			event.ExHandled = &exHandled
			event.ExAttributes = ex["attributes"]

			if frames, ok := ex["stacktrace"].([]interface{}); ok {
				event.ExStacktrace = make(model.Stacktrace, len(frames))
				for frIdx, fr := range frames {
					fr, ok := fr.(map[string]interface{})
					if !ok {
						continue
					}
					frame := model.StacktraceFrame{}
					function, _  := fr["function"].(string)
					frame.Function = &function
					absPath, _ := fr["abs_path"].(string)
					frame.AbsPath = &absPath
					frame.Filename, _ = fr["filename"].(string)
					frame.Lineno, _ = fr["lineno"].(int)
					libraryFrame, _ := fr["library_frame"].(bool)
					frame.LibraryFrame = &libraryFrame
					frame.Vars = utility.DeepGet(fr, "vars")
					module, _ := fr["module"].(string)
					frame.Module = &module
					colno, _ := fr["colno"].(int)
					frame.Colno = &colno
					contextLine, _ := fr["context_line"].(string)
					frame.ContextLine = &contextLine
					frame.PreContext, _ = fr["pre_context"].([]string)
					frame.PostContext, _ = fr["post_context"].([]string)
					event.ExStacktrace[frIdx] = &frame
				}
			} else {
				event.ExStacktrace = make(model.Stacktrace, 0)
			}

			pa.Events[errIdx] = event
		}
	} else {
		pa.Events = make([]Event, 0)
	}
	return &pa
}
