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

package model

import (
	"github.com/elastic/beats/v7/libbeat/common"
)

//Service bundles together information related to the monitored service and the agent used for monitoring
type Service struct {
	// ID          string
	Name        string
	Version     string
	Environment string
	Language    Language
	Runtime     Runtime
	Framework   Framework
	Node        ServiceNode

	// Origin *Service
}

//Language has an optional version and name
type Language struct {
	Name    string
	Version string
}

//Runtime has an optional version and name
type Runtime struct {
	Name    string
	Version string
}

//Framework has an optional version and name
type Framework struct {
	Name    string
	Version string
}

type ServiceNode struct {
	Name string
}

//Fields transforms a service instance into a common.MapStr
func (s *Service) Fields() common.MapStr {
	if s == nil {
		return nil
	}

	var svc mapStr
	// svc.maybeSetString("id", s.ID)
	svc.maybeSetString("name", s.Name)
	svc.maybeSetString("version", s.Version)
	svc.maybeSetString("environment", s.Environment)
	if node := s.Node.fields(); node != nil {
		svc.set("node", node)
	}

	var lang mapStr
	lang.maybeSetString("name", s.Language.Name)
	lang.maybeSetString("version", s.Language.Version)
	if lang != nil {
		svc.set("language", common.MapStr(lang))
	}

	var runtime mapStr
	runtime.maybeSetString("name", s.Runtime.Name)
	runtime.maybeSetString("version", s.Runtime.Version)
	if runtime != nil {
		svc.set("runtime", common.MapStr(runtime))
	}

	var framework mapStr
	framework.maybeSetString("name", s.Framework.Name)
	framework.maybeSetString("version", s.Framework.Version)
	if framework != nil {
		svc.set("framework", common.MapStr(framework))
	}

	// if s.Origin != nil {
	// 	svc.set("origin", s.Origin.Fields())
	// }

	return common.MapStr(svc)
}

func (n *ServiceNode) fields() common.MapStr {
	if n.Name != "" {
		return common.MapStr{"name": n.Name}
	}
	return nil
}
