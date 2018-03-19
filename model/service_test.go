package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

var (
	version, environment    = "5.1.3", "staging"
	langName, langVersion   = "ecmascript", "8"
	rtName, rtVersion       = "node", "8.0.0"
	fwName, fwVersion       = "Express", "1.2.3"
	agentName, agentVersion = "elastic-node", "1.0.0"
)

func TestServiceTransform(t *testing.T) {

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
					Name:    &langName,
					Version: &langVersion,
				},
				Runtime: Runtime{
					Name:    &rtName,
					Version: &rtVersion,
				},
				Framework: Framework{
					Name:    &fwName,
					Version: &fwVersion,
				},
				Agent: Agent{
					Name:    agentName,
					Version: agentVersion,
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

func TestServiceDecode(t *testing.T) {
	for _, test := range []struct {
		input interface{}
		err   error
		s     *Service
	}{
		{input: nil, err: nil, s: &Service{}},
		{input: "", err: errors.New("Invalid type for service"), s: &Service{}},
		{
			input: map[string]interface{}{"name": 1234},
			err:   errors.New("Mandatory field missing"),
			s:     &Service{},
		},
		{
			input: map[string]interface{}{
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
			err: nil,
			s: &Service{
				Name:        "myService",
				Version:     &version,
				Environment: &environment,
				Language: Language{
					Name:    &langName,
					Version: &langVersion,
				},
				Runtime: Runtime{
					Name:    &rtName,
					Version: &rtVersion,
				},
				Framework: Framework{
					Name:    &fwName,
					Version: &fwVersion,
				},
				Agent: Agent{
					Name:    agentName,
					Version: agentVersion,
				},
			},
		},
	} {
		service := &Service{}
		out := service.Decode(test.input)
		assert.Equal(t, test.s, service)
		assert.Equal(t, test.err, out)
	}
	var s *Service
	assert.Nil(t, s.Decode("a"), nil)
}
