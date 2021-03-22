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

type Container struct {
	ID        string
	Name      string
	Runtime   string
	ImageName string
	ImageTag  string
}

func (c *Container) fields() common.MapStr {
	var container mapStr
	container.maybeSetString("name", c.Name)
	container.maybeSetString("id", c.ID)
	container.maybeSetString("runtime", c.Runtime)

	var image mapStr
	image.maybeSetString("name", c.ImageName)
	image.maybeSetString("tag", c.ImageTag)
	container.maybeSetMapStr("image", common.MapStr(image))
	return common.MapStr(container)
}
