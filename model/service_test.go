package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

func TestServiceTransform(t *testing.T) {
	var service *Service

	name := "myService"
	version := "5.1.3"
	environment := "staging"
	langName := "ecmascript"
	langVersion := "8"
	rtName := "node"
	rtVersion := "8.0.0"
	fwName := "Express"
	fwVersion := "1.2.3"
	agentName := "elastic-node"
	agentVersion := "1.0.0"
	tests := []struct {
		Service *Service
		Output  common.MapStr
	}{
		{
			Service: service,
			Output:  nil,
		},
		{
			Service: &Service{},
			Output:  nil,
		},
		{
			Service: &Service{
				Name:        &name,
				Version:     &version,
				Environment: &environment,
				Language: struct {
					Name    *string
					Version *string
				}{
					Name:    &langName,
					Version: &langVersion,
				},
				Runtime: struct {
					Name    *string
					Version *string
				}{
					Name:    &rtName,
					Version: &rtVersion,
				},
				Framework: struct {
					Name    *string
					Version *string
				}{
					Name:    &fwName,
					Version: &fwVersion,
				},
				Agent: struct {
					Name    *string
					Version *string
				}{
					Name:    &agentName,
					Version: &agentVersion,
				},
			},
			Output: common.MapStr{
				"name":        &name,
				"version":     &version,
				"environment": &environment,
				"language": common.MapStr{
					"name":    &langName,
					"version": &langVersion,
				},
				"runtime": common.MapStr{
					"name":    &rtName,
					"version": &rtVersion,
				},
				"framework": common.MapStr{
					"name":    &fwName,
					"version": &fwVersion,
				},
				"agent": common.MapStr{
					"name":    &agentName,
					"version": &agentVersion,
				},
			},
		},
	}

	for _, test := range tests {
		output := test.Service.Transform()
		assert.Equal(t, test.Output, output)
	}

}
