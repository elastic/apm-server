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

	"github.com/elastic/apm-server/model/modeldecoder/nullable"
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
	AvailabilityZone nullable.String       `json:"availability_zone" validate:"max=1024"`
	Instance         metadataCloudInstance `json:"instance"`
	Machine          metadataCloudMachine  `json:"machine"`
	Project          metadataCloudProject  `json:"project"`
	Provider         nullable.String       `json:"provider" validate:"required,max=1024"`
	Region           nullable.String       `json:"region" validate:"max=1024"`
}

type metadataCloudAccount struct {
	ID   nullable.String `json:"id" validate:"max=1024"`
	Name nullable.String `json:"name" validate:"max=1024"`
}

type metadataCloudInstance struct {
	ID   nullable.String `json:"id" validate:"max=1024"`
	Name nullable.String `json:"name" validate:"max=1024"`
}

type metadataCloudMachine struct {
	Type nullable.String `json:"type" validate:"max=1024"`
}

type metadataCloudProject struct {
	ID   nullable.String `json:"id" validate:"max=1024"`
	Name nullable.String `json:"name" validate:"max=1024"`
}

type metadataProcess struct {
	Argv  []string        `json:"argv"`
	Pid   nullable.Int    `json:"pid" validate:"required"`
	Ppid  nullable.Int    `json:"ppid"`
	Title nullable.String `json:"title" validate:"max=1024"`
}

type metadataService struct {
	Agent       metadataServiceAgent     `json:"agent" validate:"required"`
	Environment nullable.String          `json:"environment" validate:"max=1024"`
	Framework   metadataServiceFramework `json:"framework"`
	Language    metadataServiceLanguage  `json:"language"`
	Name        nullable.String          `json:"name" validate:"required,max=1024,pattern=alphaNumericExtRegex"`
	Node        metadataServiceNode      `json:"node"`
	Runtime     metadataServiceRuntime   `json:"runtime"`
	Version     nullable.String          `json:"version" validate:"max=1024"`
}

type metadataServiceAgent struct {
	EphemeralID nullable.String `json:"ephemeral_id" validate:"max=1024"`
	Name        nullable.String `json:"name" validate:"required,max=1024"`
	Version     nullable.String `json:"version" validate:"required,max=1024"`
}

type metadataServiceFramework struct {
	Name    nullable.String `json:"name" validate:"max=1024"`
	Version nullable.String `json:"version" validate:"max=1024"`
}

type metadataServiceLanguage struct {
	Name    nullable.String `json:"name" validate:"required,max=1024"`
	Version nullable.String `json:"version" validate:"max=1024"`
}

type metadataServiceNode struct {
	Name nullable.String `json:"configured_name" validate:"max=1024"`
}

type metadataServiceRuntime struct {
	Name    nullable.String `json:"name" validate:"required,max=1024"`
	Version nullable.String `json:"version" validate:"required,max=1024"`
}

type metadataSystem struct {
	Architecture       nullable.String          `json:"architecture" validate:"max=1024"`
	ConfiguredHostname nullable.String          `json:"configured_hostname" validate:"max=1024"`
	Container          metadataSystemContainer  `json:"container"`
	DetectedHostname   nullable.String          `json:"detected_hostname" validate:"max=1024"`
	HostnameDeprecated nullable.String          `json:"hostname" validate:"max=1024"`
	IP                 nullable.String          `json:"ip"`
	Kubernetes         metadataSystemKubernetes `json:"kubernetes"`
	Platform           nullable.String          `json:"platform" validate:"max=1024"`
}

type metadataSystemContainer struct {
	// `id` is the only field in `system.container`,
	// if `system.container:{}` is sent, it should be considered valid
	// if additional attributes are defined in the future, add the required tag
	ID nullable.String `json:"id"` //validate:"required"
}

type metadataSystemKubernetes struct {
	Namespace nullable.String              `json:"namespace" validate:"max=1024"`
	Node      metadataSystemKubernetesNode `json:"node"`
	Pod       metadataSystemKubernetesPod  `json:"pod"`
}

type metadataSystemKubernetesNode struct {
	Name nullable.String `json:"name" validate:"max=1024"`
}

type metadataSystemKubernetesPod struct {
	Name nullable.String `json:"name" validate:"max=1024"`
	UID  nullable.String `json:"uid" validate:"max=1024"`
}

type metadataUser struct {
	ID    nullable.Interface `json:"id,omitempty" validate:"max=1024,types=string;int"`
	Email nullable.String    `json:"email" validate:"max=1024"`
	Name  nullable.String    `json:"username" validate:"max=1024"`
}

type metadataRoot struct {
	Metadata metadata `json:"metadata" validate:"required"`
}
