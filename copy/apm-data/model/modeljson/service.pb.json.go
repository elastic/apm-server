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

package modeljson

import (
	modeljson "github.com/elastic/apm-data/model/modeljson/internal"
	"github.com/elastic/apm-data/model/modelpb"
)

func ServiceModelJSON(s *modelpb.Service, out *modeljson.Service) {
	*out = modeljson.Service{
		Name:        s.Name,
		Version:     s.Version,
		Environment: s.Environment,
	}
	if s.Node != nil {
		out.Node = &modeljson.ServiceNode{
			Name: s.Node.Name,
		}
	}
	if s.Language != nil {
		out.Language = &modeljson.Language{
			Name:    s.Language.Name,
			Version: s.Language.Version,
		}
	}
	if s.Runtime != nil {
		out.Runtime = &modeljson.Runtime{
			Name:    s.Runtime.Name,
			Version: s.Runtime.Version,
		}
	}
	if s.Framework != nil {
		out.Framework = &modeljson.Framework{
			Name:    s.Framework.Name,
			Version: s.Framework.Version,
		}
	}
	if s.Origin != nil {
		out.Origin = &modeljson.ServiceOrigin{
			ID:      s.Origin.Id,
			Name:    s.Origin.Name,
			Version: s.Origin.Version,
		}
	}
	if s.Target != nil {
		out.Target = &modeljson.ServiceTarget{
			Name: s.Target.Name,
			Type: s.Target.Type,
		}
	}
}
