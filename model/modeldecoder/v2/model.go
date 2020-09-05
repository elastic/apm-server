// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package v2

import (
	"regexp"

	"github.com/elastic/apm-server/model/modeldecoder/typ"
	"github.com/elastic/beats/v7/libbeat/common"
)

var (
	alphaNumericExtRegex = regexp.MustCompile("^[a-zA-Z0-9 _-]+$")
	labelsRegex          = regexp.MustCompile("^[^.*\"]*$") //do not allow '.' '*' '"'
)

type metadata struct {
	Cloud   metadataCloud   `json:"cloud"`
	Labels  common.MapStr   `json:"labels" validate:"patternKeys=labelsRegex,typesVals=string;bool;number,maxVals=1024"`
	Process metadataProcess `json:"process"`
	Service metadataService `json:"service" validate:"required"`
	System  metadataSystem  `json:"system"`
	User    metadataUser    `json:"user"`
}

type metadataCloud struct {
	Account          metadataCloudAccount  `json:"account"`
	AvailabilityZone typ.String            `json:"availability_zone" validate:"max=1024"`
	Instance         metadataCloudInstance `json:"instance"`
	Machine          metadataCloudMachine  `json:"machine"`
	Project          metadataCloudProject  `json:"project"`
	Provider         typ.String            `json:"provider" validate:"required,max=1024"`
	Region           typ.String            `json:"region" validate:"max=1024"`
}

type metadataCloudAccount struct {
	ID   typ.String `json:"id" validate:"max=1024"`
	Name typ.String `json:"name" validate:"max=1024"`
}

type metadataCloudInstance struct {
	ID   typ.String `json:"id" validate:"max=1024"`
	Name typ.String `json:"name" validate:"max=1024"`
}

type metadataCloudMachine struct {
	Type typ.String `json:"type" validate:"max=1024"`
}

type metadataCloudProject struct {
	ID   typ.String `json:"id" validate:"max=1024"`
	Name typ.String `json:"name" validate:"max=1024"`
}

type metadataProcess struct {
	Argv  []string   `json:"argv"`
	Pid   typ.Int    `json:"pid" validate:"required"`
	Ppid  typ.Int    `json:"ppid"`
	Title typ.String `json:"title" validate:"max=1024"`
}

type metadataService struct {
	Agent       metadataServiceAgent     `json:"agent" validate:"required"`
	Environment typ.String               `json:"environment" validate:"max=1024"`
	Framework   metadataServiceFramework `json:"framework"`
	Language    metadataServiceLanguage  `json:"language"`
	Name        typ.String               `json:"name" validate:"required,max=1024,pattern=alphaNumericExtRegex"`
	Node        metadataServiceNode      `json:"node"`
	Runtime     metadataServiceRuntime   `json:"runtime"`
	Version     typ.String               `json:"version" validate:"max=1024"`
}

type metadataServiceAgent struct {
	EphemeralID typ.String `json:"ephemeral_id" validate:"max=1024"`
	Name        typ.String `json:"name" validate:"required,max=1024"`
	Version     typ.String `json:"version" validate:"required,max=1024"`
}

type metadataServiceFramework struct {
	Name    typ.String `json:"name" validate:"max=1024"`
	Version typ.String `json:"version" validate:"max=1024"`
}

type metadataServiceLanguage struct {
	Name    typ.String `json:"name" validate:"required,max=1024"`
	Version typ.String `json:"version" validate:"max=1024"`
}

type metadataServiceNode struct {
	Name typ.String `json:"configured_name" validate:"max=1024"`
}

type metadataServiceRuntime struct {
	Name    typ.String `json:"name" validate:"required,max=1024"`
	Version typ.String `json:"version" validate:"required,max=1024"`
}

type metadataSystem struct {
	Architecture       typ.String               `json:"architecture" validate:"max=1024"`
	ConfiguredHostname typ.String               `json:"configured_hostname" validate:"max=1024"`
	Container          metadataSystemContainer  `json:"container"`
	DetectedHostname   typ.String               `json:"detected_hostname" validate:"max=1024"`
	HostnameDeprecated typ.String               `json:"hostname" validate:"max=1024"`
	IP                 typ.String               `json:"ip"`
	Kubernetes         metadataSystemKubernetes `json:"kubernetes"`
	Platform           typ.String               `json:"platform" validate:"max=1024"`
}

type metadataSystemContainer struct {
	// `id` is the only field in `system.container`,
	// if `system.container:{}` is sent, it should be considered valid
	// if additional attributes are defined in the future, add the required tag
	ID typ.String `json:"id"` //validate:"required"
}

type metadataSystemKubernetes struct {
	Namespace typ.String                   `json:"namespace" validate:"max=1024"`
	Node      metadataSystemKubernetesNode `json:"node"`
	Pod       metadataSystemKubernetesPod  `json:"pod"`
}

type metadataSystemKubernetesNode struct {
	Name typ.String `json:"name" validate:"max=1024"`
}

type metadataSystemKubernetesPod struct {
	Name typ.String `json:"name" validate:"max=1024"`
	UID  typ.String `json:"uid" validate:"max=1024"`
}

type metadataUser struct {
	ID    typ.Interface `json:"id,omitempty" validate:"max=1024,types=string;int"`
	Email typ.String    `json:"email" validate:"max=1024"`
	Name  typ.String    `json:"username" validate:"max=1024"`
}

type metadataWithKey struct {
	Metadata metadata `json:"metadata" validate:"required"`
}

type metadataNoKey struct {
	Metadata metadata `validate:"required"`
}
