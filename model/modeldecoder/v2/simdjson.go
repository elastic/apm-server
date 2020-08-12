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

// comment for now to make check-full cmd happy

// import (
// 	"errors"
// 	"fmt"
// 	"sync"

// 	"github.com/minio/simdjson-go"

// 	"github.com/elastic/apm-server/decoder"
// )

// func init() {
// 	elemsPool.New = func() interface{} {
// 		elems := simdjson.Elements{
// 			Elements: make([]simdjson.Element, 0, 5),
// 			Index:    make(map[string]int, 5),
// 		}
// 		return &elems
// 	}
// 	objPool.New = func() interface{} {
// 		return &simdjson.Object{}
// 	}
// 	parsedJSONPool.New = func() interface{} {
// 		return &simdjson.ParsedJson{}
// 	}
// 	iterPool.New = func() interface{} {
// 		return &simdjson.Iter{}
// 	}
// }

// var elemsPool, iterPool, objPool, parsedJSONPool sync.Pool

// func simdjsonDecodeEmbeddedMetadata(d decoder.Decoder, out *metadata) error {
// 	return simdjsonDecodeWrappedErr(d, "", out)
// }
// func simdjsonDecodeNestedMetadata(d decoder.Decoder, out *metadata) error {
// 	return simdjsonDecodeWrappedErr(d, "metadata", out)
// }

// //TODO(simitt): look into more human readable error messages
// func simdjsonDecodeWrappedErr(d decoder.Decoder, rootKey string, out *metadata) error {
// 	if err := simdjsonDecode(d, rootKey, out); err != nil {
// 		if err == decoder.ErrLineTooLong {
// 			return err
// 		}
// 		return decoder.JSONDecodeError("data read error: " + err.Error())
// 	}
// 	return nil
// }

// func simdjsonDecode(d decoder.Decoder, rootKey string, out interface{}) error {
// 	line, err := d.Read()
// 	if err != nil {
// 		return err
// 	}
// 	reusedJSON := parsedJSONPool.Get().(*simdjson.ParsedJson)
// 	defer parsedJSONPool.Put(reusedJSON)
// 	parsedJSON, err := simdjson.Parse(line, reusedJSON)
// 	defer parsedJSONPool.Put(parsedJSON)
// 	if err != nil {
// 		return err
// 	}
// 	obj := objPool.Get().(*simdjson.Object)
// 	defer objPool.Put(obj)
// 	tmpIter := iterPool.Get().(*simdjson.Iter)
// 	defer iterPool.Put(tmpIter)
// 	if err := rootObj(parsedJSON.Iter(), tmpIter, obj); err != nil {
// 		return err
// 	}
// 	if rootKey != "" {
// 		if err := keyObject(obj, rootKey); err != nil {
// 			return err
// 		}
// 	}

// 	if err := decodeElements(obj, "", out); err != nil {
// 		return err
// 	}
// 	return nil
// }

// func rootObj(iter simdjson.Iter, tmpIter *simdjson.Iter, obj *simdjson.Object) error {
// 	iter.Advance()
// 	typ, _, err := iter.Root(tmpIter)
// 	if err != nil {
// 		return err
// 	}
// 	if typ != simdjson.TypeObject {
// 		err = errors.New("element not an object")
// 		return err
// 	}
// 	obj, err = tmpIter.Object(obj)
// 	if obj == nil {
// 		return errors.New("object not found")
// 	}
// 	return err
// }

// func keyObject(obj *simdjson.Object, key string) error {
// 	elem := obj.FindKey(key, nil)
// 	if elem == nil || elem.Type != simdjson.TypeObject {
// 		return fmt.Errorf("object with key %v not found", key)
// 	}
// 	obj, err := elem.Iter.Object(obj)
// 	if err != nil {
// 		return err
// 	}
// 	if obj == nil {
// 		return errors.New("object not found")
// 	}
// 	return err
// }

// func decodeElements(obj *simdjson.Object, key string, out interface{}) error {
// 	elems := elemsPool.Get().(*simdjson.Elements)
// 	defer elemsPool.Put(elems)
// 	_, err := obj.Parse(elems)
// 	if err != nil {
// 		return err
// 	}
// 	for _, elem := range elems.Elements {
// 		s := elem.Name
// 		//TODO(simitt): improve this by using knowledge about keys
// 		if key != "" {
// 			s = fmt.Sprintf("%s.%s", key, s)
// 		}
// 		switch elem.Type {
// 		case simdjson.TypeObject:
// 			//TODO(simitt): improve this
// 			if elem.Name == "labels" {
// 				if err := elementToDecoderModel(s, elem, out); err != nil {
// 					return err
// 				}
// 			} else {
// 				elemObj, err := elem.Iter.Object(nil)
// 				if err != nil {
// 					return err
// 				}
// 				if err := decodeElements(elemObj, s, out); err != nil {
// 					return err
// 				}
// 			}
// 		default:
// 			if err := elementToDecoderModel(s, elem, out); err != nil {
// 				return err
// 			}
// 		}
// 	}
// 	return nil
// }

// //TODO(simitt): to be generated from json tags and datatypes
// func elementToDecoderModel(key string, elem simdjson.Element, out interface{}) error {
// 	switch typ := out.(type) {
// 	case *metadata:
// 		return elementToMetadata(key, elem, typ)
// 	}
// 	return nil
// }

// //TODO(simitt): do not ignore type validation, see https://github.com/elastic/apm-server/pull/4076#discussion_r476987351
// func elementToMetadata(key string, elem simdjson.Element, m *metadata) error {
// 	switch key {
// 	case "cloud.account.id":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Cloud.Account.ID.Set(v)
// 		}
// 	case "cloud.account.name":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Cloud.Account.Name.Set(v)
// 		}
// 	case "cloud.availability_zone":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Cloud.AvailabilityZone.Set(v)
// 		}
// 	case "cloud.instance.id":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Cloud.Instance.ID.Set(v)
// 		}
// 	case "cloud.instance.name":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Cloud.Instance.Name.Set(v)
// 		}
// 	case "cloud.machine.type":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Cloud.Machine.Type.Set(v)
// 		}
// 	case "cloud.project.id":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Cloud.Project.ID.Set(v)
// 		}
// 	case "cloud.project.name":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Cloud.Project.Name.Set(v)
// 		}
// 	case "cloud.provider":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Cloud.Provider.Set(v)
// 		}
// 	case "cloud.region":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Cloud.Region.Set(v)
// 		}
// 	case "labels":
// 		if elem.Type == simdjson.TypeObject {
// 			v, err := elem.Iter.Interface()
// 			if err != nil {
// 				return err
// 			}
// 			if vMap, ok := v.(map[string]interface{}); ok {
// 				m.Labels = vMap
// 			} else {
// 				return errors.New("decoding labels failed")
// 			}
// 		}
// 	case "process.pid":
// 		if elem.Type == simdjson.TypeInt {
// 			v, err := elem.Iter.Int()
// 			if err != nil {
// 				return err
// 			}
// 			m.Process.Pid.Set(int(v))
// 		}
// 	case "process.ppid":
// 		if elem.Type == simdjson.TypeInt {
// 			v, err := elem.Iter.Int()
// 			if err != nil {
// 				return err
// 			}
// 			m.Process.Ppid.Set(int(v))
// 		}
// 	case "process.title":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Process.Title.Set(v)
// 		}
// 	case "process.argv":
// 		if elem.Type == simdjson.TypeArray {
// 			v, err := elem.Iter.Array(nil)
// 			if err != nil {
// 				return err
// 			}
// 			vStr, err := v.AsString()
// 			if err != nil {
// 				return err
// 			}
// 			m.Process.Argv = vStr
// 		}
// 	case "service.agent.ephemeral_id":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Service.Agent.EphemeralID.Set(v)
// 		}
// 	case "service.agent.name":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Service.Agent.Name.Set(v)
// 		}
// 	case "service.agent.version":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Service.Agent.Version.Set(v)
// 		}
// 	case "service.environment":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Service.Environment.Set(v)
// 		}
// 	case "service.framework.name":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Service.Framework.Name.Set(v)
// 		}
// 	case "service.framework.version":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Service.Framework.Version.Set(v)
// 		}
// 	case "service.language.name":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Service.Language.Name.Set(v)
// 		}
// 	case "service.language.version":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Service.Language.Version.Set(v)
// 		}
// 	case "service.name":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Service.Name.Set(v)
// 		}
// 	case "service.node.configured_name":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Service.Node.Name.Set(v)
// 		}
// 	case "service.runtime.name":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Service.Runtime.Name.Set(v)
// 		}
// 	case "service.runtime.version":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Service.Runtime.Version.Set(v)
// 		}
// 	case "service.version":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.Service.Version.Set(v)
// 		}
// 	case "system.architecture":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.System.Architecture.Set(v)
// 		}
// 	case "system.configured_hostname":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.System.ConfiguredHostname.Set(v)
// 		}
// 	case "system.container.id":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.System.Container.ID.Set(v)
// 		}
// 	case "system.detected_hostname":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.System.DetectedHostname.Set(v)
// 		}
// 	case "system.hostname":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.System.HostnameDeprecated.Set(v)
// 		}
// 	case "system.ip":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.System.IP.Set(v)
// 		}
// 	case "system.kubernetes.namespace":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.System.Kubernetes.Namespace.Set(v)
// 		}
// 	case "system.kubernetes.node.name":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.System.Kubernetes.Node.Name.Set(v)
// 		}
// 	case "system.kubernetes.pod.name":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.System.Kubernetes.Pod.Name.Set(v)
// 		}
// 	case "system.kubernetes.pod.uid":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.System.Kubernetes.Pod.UID.Set(v)
// 		}
// 	case "system.platform":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.System.Platform.Set(v)
// 		}
// 	case "user.id":
// 		var v interface{}
// 		var err error
// 		switch elem.Type {
// 		case simdjson.TypeString:
// 			v, err = elem.Iter.String()
// 		case simdjson.TypeInt:
// 			v, err = elem.Iter.Int()
// 		}
// 		if err != nil {
// 			return err
// 		}
// 		m.User.ID.Set(v)
// 	case "user.email":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.User.Email.Set(v)
// 		}
// 	case "user.username":
// 		if elem.Type == simdjson.TypeString {
// 			v, err := elem.Iter.String()
// 			if err != nil {
// 				return err
// 			}
// 			m.User.Name.Set(v)
// 		}
// 	}

// 	return nil
// }
