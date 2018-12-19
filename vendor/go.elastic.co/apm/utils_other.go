//+build !linux

package apm

import (
	"github.com/pkg/errors"

	sysinfo "github.com/elastic/go-sysinfo"
)

func currentProcessTitle() (string, error) {
	proc, err := sysinfo.Self()
	if err != nil {
		return "", errors.Wrap(err, "failed to get process info")
	}
	info, err := proc.Info()
	if err != nil {
		return "", errors.Wrap(err, "failed to get process info")
	}
	return info.Name, nil
}
