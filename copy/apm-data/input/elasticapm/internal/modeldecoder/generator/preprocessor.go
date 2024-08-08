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
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
)

// generateParseFlatFieldCode generates code that parses the JSON tags on a flat field,
// queries the nested field to extract the appropriate nested value which refers to the
// flat field, and finally sets the flat field value to the nested value. The code works
// under the assumption that an object is either completely flat or completely nested.
func generateParseFlatFieldCode(w io.Writer, flat structField, nested structField) error {
	nestedTag, _ := nested.tag.Lookup("json")
	flatTag, _ := flat.tag.Lookup("json")
	if !validateJSONTag(flatTag, nestedTag) {
		// invalid tags imply that flat field cannot be deduced from the nested source
		return nil
	}
	flatFieldType, err := getFlatFieldType(flat)
	if err != nil {
		return errors.Wrapf(err, "failed to get type for %s", flat.Name())
	}
	nestedSrc := nested.Name()
	nestedKey := string(flatTag[len(nestedTag)+1:]) // remove the nested tag prefix
	nestedKeyParts := strings.Split(nestedKey, ".")

	fmt.Fprintf(w, `if tmpVal, ok := val.%s["%s"]`, nestedSrc, nestedKeyParts[0])
	for i := 1; i < len(nestedKeyParts); i++ {
		fmt.Fprintf(w, `.(map[string]interface{}); ok {
			if tmpVal, ok := tmpVal["%s"]`, nestedKeyParts[i])
	}
	fmt.Fprintf(w, `.(%s); ok {`, flatFieldType)
	extraCode, finalVar := generateValueConverterCode(flatFieldType, "tmpVal")
	fmt.Fprintf(w, `
		%sval.%s.Set(%s)
	`, extraCode, flat.Name(), finalVar)

	// Close all braces
	for i := 0; i < len(nestedKeyParts); i++ {
		fmt.Fprintln(w, "}")
	}
	return nil
}

// validateJSONTag checks if the flat field can be extracted from the nested source
// by validating the JSON tag.
func validateJSONTag(flatTag, nestedTag string) bool {
	return flatTag != "" && nestedTag != "" && strings.HasPrefix(flatTag, nestedTag)
}

func getFlatFieldType(f structField) (string, error) {
	switch typ := f.Type().String(); typ {
	case nullableTypeInt, nullableTypeInt64:
		return "json.Number", nil
	case nullableTypeString:
		return "string", nil
	case nullableTypeBool:
		return "bool", nil
	default:
		return "", fmt.Errorf("not implemented for type: %s", typ)
	}
}

// generateValueConverterCode takes the field type of the final
// parsed value in the map and returns extra code required to
// convert the value for the flat field. It also returns a new
// variable string as the second argument if the processing requires
// to use a new variable.
func generateValueConverterCode(typ, origVar string) (string, string) {
	switch typ {
	case "json.Number":
		return fmt.Sprintf(`
			%[1]s, err := %[1]s.Int64()
			if err != nil {
				return err
			}
			newVal := int(%[1]s)
		`[1:], origVar), "newVal"
	default:
		return "", origVar
	}
}
