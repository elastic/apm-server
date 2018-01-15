package sourcemap

import (
	"fmt"
	"strings"
)

type Id struct {
	ServiceName    string
	ServiceVersion string
	Path           string
}

func (i *Id) Key() string {
	info := []string{}
	info = add(info, i.ServiceName)
	info = add(info, i.ServiceVersion)
	info = add(info, i.Path)
	return strings.Join(info, "_")
}

func (i *Id) String() string {
	return fmt.Sprintf("Service Name: '%s', Service Version: '%s' and Path: '%s'",
		i.ServiceName,
		i.ServiceVersion,
		i.Path)
}

func (i *Id) Valid() bool {
	return i.ServiceName != "" && i.ServiceVersion != "" && i.Path != ""
}

func add(a []string, s string) []string {
	if s != "" {
		a = append(a, s)
	}
	return a
}
