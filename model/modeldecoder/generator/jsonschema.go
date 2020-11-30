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

package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// JSONSchemaGenerator holds the parsed package information
// from which it can generate a JSON schema
type JSONSchemaGenerator struct {
	parsed *Parsed
}

// NewJSONSchemaGenerator takes a parsed package information as
// input parameter and returns a JSONSchemaGenerator instance
func NewJSONSchemaGenerator(parsed *Parsed) (*JSONSchemaGenerator, error) {
	return &JSONSchemaGenerator{parsed: parsed}, nil
}

// Generate creates a JSON schema for the given root type,
// based on the parsed information from the JSONSchemaGenerator.
// The schema is returned as bytes.Buffer
func (g *JSONSchemaGenerator) Generate(idPath string, rootType string) (bytes.Buffer, error) {
	root, ok := g.parsed.structTypes[rootType]
	if !ok {
		return bytes.Buffer{}, fmt.Errorf("object with root key %s not found", rootType)
	}
	typ := propertyType{names: []propertyTypeName{TypeNameObject}, required: true}
	property := property{Type: &typ, Properties: make(map[string]*property), Description: root.comment}
	if err := g.generate(root, "", &property); err != nil {
		return bytes.Buffer{}, errors.Wrap(err, "json-schema generator")
	}
	id := filepath.Join(idPath, strings.TrimSuffix(root.name, "Event"))
	b, err := json.MarshalIndent(schema{ID: id, property: property}, "", "  ")
	return *bytes.NewBuffer(b), errors.Wrap(err, "json-schema generator")
}

func (g *JSONSchemaGenerator) generate(st structType, key string, prop *property) error {
	if key != "" {
		key += "."
	}
	for _, f := range st.fields {
		var err error
		name := jsonSchemaName(f)
		childProp := property{Properties: make(map[string]*property), Type: &propertyType{}, Description: f.comment}
		tags, err := validationTag(f.tag)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("%s%s", key, name))
		}
		// handle generic tag rules applicable to all types
		if _, ok := tags[tagRequired]; ok {
			childProp.Type.required = true
			prop.Required = append(prop.Required, name)
			delete(tags, tagRequired)
		}
		if val, ok := tags[tagRequiredIfAny]; ok {
			// add all property names as entries to the property's AllOf collection
			// after processing all child fields iterate through the AllOf collection
			// and ensure the types for the properties in the If and Then clauses do not allow null values
			// this can only be done after all fields have been processed and their types are known
			for _, ifGiven := range strings.Split(val, ";") {
				prop.AllOf = append(prop.AllOf,
					&property{
						If:   &property{Required: []string{ifGiven}},
						Then: &property{Required: []string{name}}})
			}
			delete(tags, tagRequiredIfAny)
		}
		if val, ok := tags[tagRequiredAnyOf]; ok {
			// add all property names as entries to the property's AnyOf collection
			// after processing all child fields iterate through the AnyOf collection
			// and ensure the types for the properties do not allow null values
			// this can only be done after all fields have been processed and their types are known
			for _, anyOf := range strings.Split(val, ";") {
				prop.AnyOf = append(prop.AnyOf, &property{Required: []string{anyOf}})
			}
		}

		if !f.Exported() {
			continue
		}
		flattenedName := fmt.Sprintf("%s%s", key, name)

		info := fieldInfo{field: f, tags: tags, parsed: g.parsed}
		switch f.Type().String() {
		case nullableTypeBool:
			err = generateJSONPropertyBool(&info, prop, &childProp)
		case nullableTypeFloat64:
			err = generateJSONPropertyJSONNumber(&info, prop, &childProp)
		case nullableTypeHTTPHeader:
			err = generateJSONPropertyHTTPHeader(&info, prop, &childProp)
		case nullableTypeInt, nullableTypeTimeMicrosUnix:
			err = generateJSONPropertyInteger(&info, prop, &childProp)
		case nullableTypeInterface:
			err = generateJSONPropertyInterface(&info, prop, &childProp)
		case nullableTypeString:
			err = generateJSONPropertyString(&info, prop, &childProp)
		default:
			switch t := f.Type().Underlying().(type) {
			case *types.Map:
				nestedProp := property{Properties: make(map[string]*property)}
				if err = generateJSONPropertyMap(&info, prop, &childProp, &nestedProp); err != nil {
					break
				}
				if childStruct, ok := g.customStruct(t.Elem()); ok {
					err = g.generate(childStruct, flattenedName, &nestedProp)
				}
			case *types.Slice:
				if err = generateJSONPropertySlice(&info, prop, &childProp); err != nil {
					break
				}
				child, ok := g.parsed.structTypes[t.Elem().String()]
				if !ok {
					break
				}
				if child.name == st.name {
					// if recursive reference to struct itself, set object type and do not call generate function
					childProp.Items = &property{Type: &propertyType{names: []propertyTypeName{TypeNameObject}}}
					break
				}
				err = g.generate(child, flattenedName, &childProp)
			case *types.Struct:
				if err = generateJSONPropertyStruct(&info, prop, &childProp); err != nil {
					break
				}
				// all non-parsed struct types should have been handled at this point
				child, ok := g.parsed.structTypes[f.Type().String()]
				if !ok {
					err = fmt.Errorf("unhandled type for field %s", name)
					break
				}
				err = g.generate(child, flattenedName, &childProp)
			default:
				err = fmt.Errorf("unhandled type %T", t)
			}
		}
		if err != nil {
			return errors.Wrap(err, flattenedName)
		}
		for tagName := range tags {
			// not all tags have been handled
			return errors.Wrap(fmt.Errorf("unhandled tag rule %s", tagName), jsonSchemaName(f))
		}
	}

	// iterate through AnyOf and ensure that at least one value is required
	for i := 0; i < len(prop.AnyOf); i++ {
		prop.AnyOf[i].Properties = make(map[string]*property)
		for _, required := range prop.AnyOf[i].Required {
			p, ok := prop.Properties[required]
			if !ok {
				return errors.Wrap(fmt.Errorf("unhandled property %s in %s tag", required, tagRequiredAnyOf), key)
			}
			prop.AnyOf[i].Properties[required] = &property{Type: &propertyType{required: true, names: p.Type.names}}
		}
	}
	for i := 0; i < len(prop.AllOf); i++ {
		pAllOf := prop.AllOf[i]
		// set type to required for If branch
		prop.AllOf[i].If.Properties = make(map[string]*property)
		for _, required := range pAllOf.If.Required {
			p, ok := prop.Properties[required]
			if !ok {
				return errors.Wrap(fmt.Errorf("unhandled property %s in %s tag", required, tagRequiredIfAny), key)
			}
			prop.AllOf[i].If.Properties[required] = &property{Type: &propertyType{required: true, names: p.Type.names}}
		}
		// set type to required for Then branch
		prop.AllOf[i].Then.Properties = make(map[string]*property)
		for _, required := range pAllOf.Then.Required {
			p, ok := prop.Properties[required]
			if !ok {
				return errors.Wrap(fmt.Errorf("unhandled property %s in %s tag", required, tagRequiredIfAny), key)
			}
			prop.AllOf[i].Then.Properties[required] = &property{Type: &propertyType{required: true, names: p.Type.names}}
		}
	}
	return nil
}

var (
	patternHTTPHeaders = "[.*]*$"

	propertyTypes = map[string]propertyTypeName{
		nullableTypeBool:           TypeNameBool,
		"bool":                     TypeNameBool,
		nullableTypeFloat64:        TypeNameNumber,
		"number":                   TypeNameNumber,
		"float64":                  TypeNameNumber,
		nullableTypeInt:            TypeNameInteger,
		"int":                      TypeNameInteger,
		nullableTypeTimeMicrosUnix: TypeNameInteger,
		nullableTypeString:         TypeNameString,
		"string":                   TypeNameString,
		nullableTypeHTTPHeader:     TypeNameObject,
		"object":                   TypeNameObject,
	}
)

type fieldInfo struct {
	field  structField
	tags   map[string]string
	parsed *Parsed
}

type schema struct {
	ID string `json:"$id,omitempty"`
	property
}

type property struct {
	Description string        `json:"description,omitempty"`
	Type        *propertyType `json:"type,omitempty"`
	// AdditionalProperties should default to `true` and be set to `false`
	// in case PatternProperties are set
	AdditionalProperties interface{}          `json:"additionalProperties,omitempty"`
	PatternProperties    map[string]*property `json:"patternProperties,omitempty"`
	Properties           map[string]*property `json:"properties,omitempty"`
	Items                *property            `json:"items,omitempty"`
	Required             []string             `json:"required,omitempty"`
	Enum                 []*string            `json:"enum,omitempty"`
	Max                  json.Number          `json:"maximum,omitempty"`
	MaxLength            json.Number          `json:"maxLength,omitempty"`
	Min                  json.Number          `json:"minimum,omitempty"`
	MinItems             *int                 `json:"minItems,omitempty"`
	MinLength            json.Number          `json:"minLength,omitempty"`
	Pattern              string               `json:"pattern,omitempty"`
	AllOf                []*property          `json:"allOf,omitempty"`
	AnyOf                []*property          `json:"anyOf,omitempty"`
	If                   *property            `json:"if,omitempty"`
	Then                 *property            `json:"then,omitempty"`
}

type propertyType struct {
	names    []propertyTypeName
	required bool
}

func (t *propertyType) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString("")
	if len(t.names) == 0 && !t.required {
		buffer.WriteString(`""`)
		return buffer.Bytes(), nil
	}
	multipleTypes := !t.required || len(t.names) > 1
	if multipleTypes {
		buffer.WriteString(`[`)
	}
	if !t.required {
		buffer.WriteString(`"null",`)
	}
	for i := 0; i < len(t.names); i++ {
		if i > 0 {
			buffer.WriteString(`,`)
		}
		buffer.WriteString(`"`)
		buffer.WriteString(t.names[i].String())
		buffer.WriteString(`"`)
	}
	if multipleTypes {
		buffer.WriteString(`]`)
	}
	return buffer.Bytes(), nil
}

func (t *propertyType) add(name propertyTypeName) {
	if t.contains(name) {
		return
	}
	t.names = append(t.names, name)
}

func (t *propertyType) contains(name propertyTypeName) bool {
	for _, n := range t.names {
		if n == name {
			return true
		}
	}
	return false
}

type propertyTypeName uint8

//go:generate stringer -linecomment -type propertyTypeName
const (
	//TypeNameObject represents an object
	TypeNameObject propertyTypeName = iota //object
	//TypeNameString represents an string
	TypeNameString //string
	//TypeNameBool represents an boolean
	TypeNameBool //boolean
	//TypeNameInteger represents an integer
	TypeNameInteger //integer
	//TypeNameNumber represents a number (float or integer)
	TypeNameNumber //number
	//TypeNameArray represents and object
	TypeNameArray //array
)

func propertyTypesFromTag(tagName string, tagValue string) ([]propertyTypeName, error) {
	names := strings.Split(tagValue, ";")
	types := make([]propertyTypeName, len(names))
	var ok bool
	for i := 0; i < len(names); i++ {
		if types[i], ok = propertyTypes[names[i]]; !ok {
			return nil, fmt.Errorf("unhandled value %s for tag %s", names[i], tagName)
		}
	}
	return types, nil
}

func (g *JSONSchemaGenerator) customStruct(typ types.Type) (t structType, ok bool) {
	t, ok = g.parsed.structTypes[typ.String()]
	return
}

func jsonSchemaName(f structField) string {
	parts := parseTag(f.tag, "json")
	if parts == nil {
		return ""
	}
	if len(parts) == 0 {
		return strings.ToLower(f.Name())
	}
	return parts[0]
}
