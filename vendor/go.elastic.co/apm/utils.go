package apm

import (
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"

	"go.elastic.co/apm/internal/apmhostutil"
	"go.elastic.co/apm/internal/apmstrings"
	"go.elastic.co/apm/model"
)

var (
	currentProcess model.Process
	goAgent        = model.Agent{Name: "go", Version: AgentVersion}
	goLanguage     = model.Language{Name: "go", Version: runtime.Version()}
	goRuntime      = model.Runtime{Name: runtime.Compiler, Version: runtime.Version()}
	localSystem    model.System

	serviceNameInvalidRegexp = regexp.MustCompile("[^" + serviceNameValidClass + "]")
	tagKeyReplacer           = strings.NewReplacer(`.`, `_`, `*`, `_`, `"`, `_`)
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
	if err != nil || title == "" {
		title = filepath.Base(os.Args[0])
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
		Agent:       &goAgent,
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
	if container, err := apmhostutil.Container(); err == nil {
		system.Container = container
	}
	system.Kubernetes = getKubernetesMetadata()
	return system
}

func getKubernetesMetadata() *model.Kubernetes {
	kubernetes, err := apmhostutil.Kubernetes()
	if err != nil {
		kubernetes = nil
	}
	namespace := os.Getenv("KUBERNETES_NAMESPACE")
	podName := os.Getenv("KUBERNETES_POD_NAME")
	podUID := os.Getenv("KUBERNETES_POD_UID")
	nodeName := os.Getenv("KUBERNETES_NODE_NAME")
	if namespace == "" && podName == "" && podUID == "" && nodeName == "" {
		return kubernetes
	}
	if kubernetes == nil {
		kubernetes = &model.Kubernetes{}
	}
	if namespace != "" {
		kubernetes.Namespace = namespace
	}
	if nodeName != "" {
		if kubernetes.Node == nil {
			kubernetes.Node = &model.KubernetesNode{}
		}
		kubernetes.Node.Name = nodeName
	}
	if podName != "" || podUID != "" {
		if kubernetes.Pod == nil {
			kubernetes.Pod = &model.KubernetesPod{}
		}
		if podName != "" {
			kubernetes.Pod.Name = podName
		}
		if podUID != "" {
			kubernetes.Pod.UID = podUID
		}
	}
	return kubernetes
}

func cleanTagKey(k string) string {
	return tagKeyReplacer.Replace(k)
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
	// At the time of writing, all keyword length
	// limits are 1024, enforced by JSON Schema.
	return apmstrings.Truncate(s, 1024)
}

func truncateLongString(s string) string {
	// Non-keyword string fields are not limited
	// in length by JSON Schema, but we still
	// truncate all strings. Some strings, such
	// as database statement, we explicitly allow
	// to be longer than others.
	return apmstrings.Truncate(s, 10000)
}

func nextGracePeriod(p time.Duration) time.Duration {
	if p == -1 {
		return 0
	}
	for i := time.Duration(0); i < 6; i++ {
		if p == (i * i * time.Second) {
			return (i + 1) * (i + 1) * time.Second
		}
	}
	return p
}

// jitterDuration returns d +/- some multiple of d in the range [0,j].
func jitterDuration(d time.Duration, rng *rand.Rand, j float64) time.Duration {
	if d == 0 || j == 0 {
		return d
	}
	r := (rng.Float64() * j * 2) - j
	return d + time.Duration(float64(d)*r)
}
