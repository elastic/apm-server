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
			methodName = fmt.Sprintf("%s:%s", methodName, sourceFileName)
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
	//var currentMappedMethodCall *MappedMethodCall

	for scanner.Scan() {
		line := scanner.Text()
		typeMatch := typePattern.FindStringSubmatch(line)
		if typeMatch != nil {
			//currentMappedMethodCall = nil
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
				methodObfuscatedName := methodMatch[4]
				methodRealName := methodMatch[3]
				frames, ok := currentType.obfuscated.methods[methodObfuscatedName]
				if ok {
					for _, frame := range frames {
						frame.Original.Function = methodObfuscatedName
						frame.Function = methodRealName
					}
				}

				//currentMappedMethodCall = upsertMappedMethodCall(mapping, MethodMatch{
				//	sourceFileStart:      methodMatch[1],
				//	sourceFileEnd:        methodMatch[2],
				//	methodRealName:       methodMatch[3],
				//	methodObfuscatedName: methodMatch[4],
				//}, currentType, currentMappedMethodCall)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

// Checks if the found methodMatch from the map file is part of the methods previously parsed for the currentType within the
// stacktrace.
// If the method is found, then it's added to the mapping in order to be used for replacing the obfuscated name later, it also returns it as the current MappedMethodCall.
// If the method is NOT found, but it turns out to be a continuation of a previously found method (currentMappedMethodCall), then it appends it to the existing method replacement in the mapping.
// If the method is NOT found and is also NOT a continuation for the currentMappedMethodCall, then the methodMatch is ignored.
func upsertMappedMethodCall(mapping map[string]string, methodMatch MethodMatch, currentType *MappedType, currentMappedMethodCall *MappedMethodCall) *MappedMethodCall {
	//methodNameReference := methodMatch.methodObfuscatedName
	//if methodMatch.sourceFileStart != "" {
	//	if methodMatch.sourceFileStart != methodMatch.sourceFileEnd {
	//		// This is probably due an edge-case where the mapping line starts with different numbers (e.g 1:2). We don't
	//		// have that case in our tests, therefore we are ignoring it.
	//		return currentMappedMethodCall
	//	}
	//	methodNameReference = fmt.Sprintf("%s:%s", methodMatch.methodObfuscatedName, methodMatch.sourceFileStart)
	//}
	//mapReference := currentType.obfuscated.name + ":" + methodNameReference
	//methodCallSite, ok := currentType.obfuscated.methods[methodNameReference]
	//if ok {
	//	Found this method in the list of methods parsed from the stacktrace for the currentType.
	//delete(currentType.obfuscated.methods, methodNameReference)
	//key := getKey(currentType.obfuscated.name, methodMatch.methodObfuscatedName, methodCallSite)
	//mapping[key] = getKey(currentType.realName, methodMatch.methodRealName, methodCallSite)
	//return &MappedMethodCall{reference: mapReference, key: key}
	//} else if currentMappedMethodCall != nil && currentMappedMethodCall.reference == mapReference {
	//	This is a continuation call for the currentMappedMethodCall.
	//	mapping[currentMappedMethodCall.key] += "\n" + fmt.Sprintf("%s%s", strings.Repeat(" ", len(currentType.realName)+currentType.obfuscated.indentation), methodMatch.methodRealName)
	//}

	return currentMappedMethodCall
}
