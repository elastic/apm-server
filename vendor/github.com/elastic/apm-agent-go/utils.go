package elasticapm

import (
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/apm-agent-go/internal/apmstrings"
	"github.com/elastic/apm-agent-go/model"
)

var (
	currentProcess model.Process
	goAgent        = model.Agent{Name: "go", Version: AgentVersion}
	goLanguage     = model.Language{Name: "go", Version: runtime.Version()}
	goRuntime      = model.Runtime{Name: runtime.Compiler, Version: runtime.Version()}
	localSystem    model.System

	serviceNameInvalidRegexp = regexp.MustCompile("[^" + serviceNameValidClass + "]")
)

const (
	envHostname = "ELASTIC_APM_HOSTNAME"

	serviceNameValidClass = "a-zA-Z0-9 _-"
)

func init() {
	currentProcess = getCurrentProcess()
	localSystem = getLocalSystem()
}

func getCurrentProcess() model.Process {
	ppid := os.Getppid()
	title, err := currentProcessTitle()
	if err != nil {
		title = os.Args[0]
	}
	return model.Process{
		Pid:   os.Getpid(),
		Ppid:  &ppid,
		Title: truncateString(title),
		Argv:  os.Args,
	}
}

func makeService(name, version, environment string) model.Service {
	return model.Service{
		Name:        truncateString(name),
		Version:     truncateString(version),
		Environment: truncateString(environment),
		Agent:       goAgent,
		Language:    &goLanguage,
		Runtime:     &goRuntime,
	}
}

func getLocalSystem() model.System {
	system := model.System{
		Architecture: runtime.GOARCH,
		Platform:     runtime.GOOS,
	}
	system.Hostname = os.Getenv(envHostname)
	if system.Hostname == "" {
		if hostname, err := os.Hostname(); err == nil {
			system.Hostname = hostname
		}
	}
	system.Hostname = truncateString(system.Hostname)
	return system
}

func validTagKey(k string) bool {
	return !strings.ContainsAny(k, `.*"`)
}

func validateServiceName(name string) error {
	idx := serviceNameInvalidRegexp.FindStringIndex(name)
	if idx == nil {
		return nil
	}
	return errors.Errorf(
		"invalid service name %q: character %q is not in the allowed set (%s)",
		name, name[idx[0]], serviceNameValidClass,
	)
}

func sanitizeServiceName(name string) string {
	return serviceNameInvalidRegexp.ReplaceAllString(name, "_")
}

func truncateString(s string) string {
	// At the time of writing, all length limits are 1024.
	return apmstrings.Truncate(s, 1024)
}
