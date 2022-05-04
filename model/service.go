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
	"github.com/elastic/elastic-agent-libs/mapstr"
)

// Service bundles together information related to the monitored service and
// the agent used for monitoring
type Service struct {
	Name        string
	Version     string
	Environment string
	Language    Language
	Runtime     Runtime
	Framework   Framework
	Node        ServiceNode

	Origin *ServiceOrigin
	Target *ServiceTarget
}

// ServiceOrigin holds information about the service that originated a
// transaction.
type ServiceOrigin struct {
	ID      string
	Name    string
	Version string
}

// ServiceTarget holds information about the target service in case of
// an outgoing event w.r.t. the instrumented service
type ServiceTarget struct {
	Name string
	Type string
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

//Fields transforms a service instance into a mapstr.M
func (s *Service) Fields() mapstr.M {
	if s == nil {
		return nil
	}

	var svc mapStr
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
		svc.set("language", mapstr.M(lang))
	}

	var runtime mapStr
	runtime.maybeSetString("name", s.Runtime.Name)
	runtime.maybeSetString("version", s.Runtime.Version)
	if runtime != nil {
		svc.set("runtime", mapstr.M(runtime))
	}

	var framework mapStr
	framework.maybeSetString("name", s.Framework.Name)
	framework.maybeSetString("version", s.Framework.Version)
	if framework != nil {
		svc.set("framework", mapstr.M(framework))
	}

	if s.Origin != nil {
		var origin mapStr
		origin.maybeSetString("name", s.Origin.Name)
		origin.maybeSetString("version", s.Origin.Version)
		origin.maybeSetString("id", s.Origin.ID)
		svc.maybeSetMapStr("origin", mapstr.M(origin))
	}

	if s.Target != nil {
		var target mapStr
		target.maybeSetString("name", s.Target.Name)
		target.maybeSetString("type", s.Target.Type)
		svc.maybeSetMapStr("target", mapstr.M(target))
	}

	return mapstr.M(svc)
}

func (n *ServiceNode) fields() mapstr.M {
	if n.Name != "" {
		return mapstr.M{"name": n.Name}
	}
	return nil
}
