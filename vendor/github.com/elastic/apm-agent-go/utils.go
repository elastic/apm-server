package elasticapm

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/apm-agent-go/model"
)

var (
	currentProcess model.Process
	envFramework   *model.Framework
	envService     model.Service
	goAgent        = model.Agent{Name: "go", Version: AgentVersion}
	goLanguage     = model.Language{Name: "go", Version: runtime.Version()}
	goRuntime      = model.Runtime{Name: runtime.Compiler, Version: runtime.Version()}
	localSystem    model.System

	serviceNameInvalidRegexp = regexp.MustCompile("[^" + serviceNameValidClass + "]")
)

const (
	envEnvironment      = "ELASTIC_APM_ENVIRONMENT"
	envFrameworkName    = "ELASTIC_APM_FRAMEWORK_NAME"
	envFrameworkVersion = "ELASTIC_APM_FRAMEWORK_VERSION"
	envHostname         = "ELASTIC_APM_HOSTNAME"
	envServiceName      = "ELASTIC_APM_SERVICE_NAME"
	envServiceVersion   = "ELASTIC_APM_SERVICE_VERSION"

	serviceNameValidClass = "a-zA-Z0-9 _-"
)

func init() {
	currentProcess = getCurrentProcess()
	envFramework = getEnvironmentFramework()
	envService = getEnvironmentService()
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
		Title: title,
		Argv:  os.Args,
	}
}

func getEnvironmentFramework() *model.Framework {
	name := os.Getenv(envFrameworkName)
	if name == "" {
		return nil
	}
	return &model.Framework{
		Name:    name,
		Version: os.Getenv(envFrameworkVersion),
	}
}

func getEnvironmentService() model.Service {
	name := os.Getenv(envServiceName)
	if name == "" {
		name = filepath.Base(os.Args[0])
		if runtime.GOOS == "windows" {
			name = strings.TrimSuffix(name, filepath.Ext(name))
		}
	}
	svc := newService(sanitizeServiceName(name), "")
	return *svc
}

func newService(name, version string) *model.Service {
	if version == "" {
		version = os.Getenv(envServiceVersion)
	}
	return &model.Service{
		Name:        name,
		Version:     version,
		Environment: os.Getenv(envEnvironment),
		Agent:       goAgent,
		Framework:   envFramework,
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
