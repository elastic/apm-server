package apmhostutil

import "go.elastic.co/apm/model"

// Container returns information about the container running the process, or an
// error the container information could not be determined.
func Container() (*model.Container, error) {
	return containerInfo()
}

// Kubernetes returns information about the Kubernetes node and pod running
// the process, or an error if they could not be determined. This information
// does not include the KUBERNETES_* environment variables that can be set via
// the Downward API.
func Kubernetes() (*model.Kubernetes, error) {
	return kubernetesInfo()
}
