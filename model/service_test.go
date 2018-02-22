package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

func TestServiceTransformDefinition(t *testing.T) {
	myfn := func(fn TransformService) string { return "ok" }
	res := myfn((*Service).Transform)
	assert.Equal(t, "ok", res)
}

func TestServiceTransform(t *testing.T) {

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
		Service Service
		Output  common.MapStr
	}{
		{
			Service: Service{},
			Output: common.MapStr{
				"agent": common.MapStr{
					"name":    "",
					"version": "",
				},
				"name": "",
			},
		},
		{
			Service: Service{
				Name:        "myService",
				Version:     &version,
				Environment: &environment,
				Language: Language{
					LanguageName:    &langName,
					LanguageVersion: &langVersion,
				},
				Runtime: Runtime{
					RuntimeName:    &rtName,
					RuntimeVersion: &rtVersion,
				},
				Framework: Framework{
					FrameworkName:    &fwName,
					FrameworkVersion: &fwVersion,
				},
				Agent: Agent{
					AgentName:    agentName,
					AgentVersion: agentVersion,
				},
			},
			Output: common.MapStr{
				"name":        "myService",
				"version":     "5.1.3",
				"environment": "staging",
				"language": common.MapStr{
					"name":    "ecmascript",
					"version": "8",
				},
				"runtime": common.MapStr{
					"name":    "node",
					"version": "8.0.0",
				},
				"framework": common.MapStr{
					"name":    "Express",
					"version": "1.2.3",
				},
				"agent": common.MapStr{
					"name":    "elastic-node",
					"version": "1.0.0",
				},
			},
		},
	}

	for _, test := range tests {
		output := test.Service.Transform()
		assert.Equal(t, test.Output, output)
	}

}
