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

package r8

import (
	"bufio"
	"fmt"
	"io"
	"regexp"

	"github.com/elastic/apm-data/model"
)

type StacktraceType struct {
	name    string
	frames  []*model.StacktraceFrame
	methods map[string][]*model.StacktraceFrame // Maps method references to their stacktrace call site
}

type MappedType struct {
	obfuscated StacktraceType
	realName   string
}

type MappedMethodCall struct {
	reference string // For simple obfuscated methods, it's the method name. For multiline method calls, it's methodName:sourceFileNumber. E.g. for a method named "a" with (SourceFile:4) reference = "a:4"
	key       string
}

type MethodMatch struct {
	sourceFileStart      string
	sourceFileEnd        string
	methodRealName       string
	methodObfuscatedName string
}

var (
	typePattern   = regexp.MustCompile(`^(\S+) -> (\S+):$`)
	methodPattern = regexp.MustCompile(`(?:(\d+):(\d+):)*\S+ (\S+)\(.*\)(?:[:\d]+)* -> (\S+)`)
)

// Deobfuscate parses the stacktrace looking for type names and their methods, then searches for those stacktrace items through the mapFile, looking
// for their de-obfuscated names to later replace the ones in the original stacktrace by their real names found within the mapFile.
func Deobfuscate(stacktrace *model.Stacktrace, mapFile io.Reader) error {
	types, err := findUniqueTypes(stacktrace)
	if err != nil {
		return err
	}
	err = findMappingFor(types, mapFile)
	if err != nil {
		return err
	}

	return nil
}

func findUniqueTypes(stacktrace *model.Stacktrace) (map[string]StacktraceType, error) {
	var symbols = make(map[string]StacktraceType)

	for _, frame := range *stacktrace {
		typeName := frame.Classname
		methodName := frame.Function
		sourceFileName := frame.Filename
		if sourceFileName == "SourceFile" {
			// Sometimes a method call in the stacktrace might end with (SourceFile:N), where N is an int. When this happens,
			// it means that the de-obfuscated version of this method starts with "N:N" in the map file. So in those cases,
			// we append the N to the method name M so that M:N becomes a "method reference" that we can later spot
			// when looping through the map file looking for the de-obfuscated names.
			methodName = fmt.Sprintf("%s:%d", methodName, *frame.Lineno)
		}
		symbol, ok := symbols[typeName]
		if !ok {
			symbol = StacktraceType{
				name:    typeName,
				frames:  make([]*model.StacktraceFrame, 0),
				methods: make(map[string][]*model.StacktraceFrame),
			}
			symbols[typeName] = symbol
		}
		symbol.frames = append(symbol.frames, frame)
		_, ok = symbol.methods[methodName]
		if !ok {
			symbol.methods[methodName] = make([]*model.StacktraceFrame, 0)
		}
		symbol.methods[methodName] = append(symbol.methods[methodName], frame)

		symbols[typeName] = symbol
	}

	return symbols, nil
}

func findMappingFor(symbols map[string]StacktraceType, mapReader io.Reader) error {
	scanner := bufio.NewScanner(mapReader)
	var currentType *MappedType

	for scanner.Scan() {
		line := scanner.Text()
		typeMatch := typePattern.FindStringSubmatch(line)
		if typeMatch != nil {
			obfuscatedName := typeMatch[2]
			stacktraceType, ok := symbols[obfuscatedName]
			if ok {
				currentType = &MappedType{stacktraceType, typeMatch[1]}
				for _, frame := range stacktraceType.frames {
					frame.Original.Classname = obfuscatedName
					frame.Classname = typeMatch[1]
					frame.SourcemapUpdated = true
				}
			} else {
				currentType = nil
			}
		} else if currentType != nil {
			methodMatch := methodPattern.FindStringSubmatch(line)
			if methodMatch != nil {
				sourceFileStart := methodMatch[1]
				sourceFileEnd := methodMatch[2]
				methodObfuscatedName := methodMatch[4]
				methodRealName := methodMatch[3]
				methodKey := methodObfuscatedName
				if sourceFileStart != "" && sourceFileStart == sourceFileEnd {
					methodKey += ":" + sourceFileStart
				}
				frames, ok := currentType.obfuscated.methods[methodKey]
				if ok {
					for _, frame := range frames {
						if frame.Original.Function == "" {
							frame.Original.Function = methodObfuscatedName
							frame.Function = methodRealName
						} else {
							frame.Function += "\n" + methodRealName
						}
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}
