// +build !linux

package apmhostutil

import (
	"runtime"

	"github.com/pkg/errors"

	"go.elastic.co/apm/model"
)

func containerInfo() (*model.Container, error) {
	return nil, errors.Errorf("container ID computation not implemented for %s", runtime.GOOS)
}

func kubernetesInfo() (*model.Kubernetes, error) {
	return nil, errors.Errorf("kubernetes info gathering not implemented for %s", runtime.GOOS)
}
