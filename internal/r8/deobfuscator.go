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

	"github.com/elastic/apm-data/model/modelpb"
)

// StacktraceType groups frames belonging to a single class as well as frames that represent the same method call.
type StacktraceType struct {
	methods map[string][]*modelpb.StacktraceFrame // Maps method references to all the frames where it appears.
}

var (
	typePattern   = regexp.MustCompile(`^(\S+) -> (\S+):$`)
	methodPattern = regexp.MustCompile(`(?:(\d+):(\d+):)*\S+ (\S+)\(.*\)(?:[:\d]+)* -> (\S+)`)
)

// Deobfuscate mutates the stacktrace by searching for those items through the mapFile, looking
// for their de-obfuscated names and replacing the ones in the original stacktrace by their real names found within the mapFile.
// Note that not all the stacktrace items might be present in the mapFile, for those cases, those frames will remain untouched.
func Deobfuscate(stacktrace *[]*modelpb.StacktraceFrame, mapFile io.Reader) error {
	types, err := groupUniqueTypes(stacktrace)
	if err != nil {
		return err
	}
	err = resolveMappings(types, mapFile)
	if err != nil {
		return err
	}

	return nil
}

// Iterates over the stacktrace and groups the frames by classname, along with its methods, which are grouped by
// the method name.
func groupUniqueTypes(stacktrace *[]*modelpb.StacktraceFrame) (map[string]StacktraceType, error) {
	var types = make(map[string]StacktraceType)

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
		typeItem, ok := types[typeName]
		if !ok {
			typeItem = StacktraceType{
				methods: make(map[string][]*modelpb.StacktraceFrame),
			}
			types[typeName] = typeItem
		}
		_, ok = typeItem.methods[methodName]
		if !ok {
			typeItem.methods[methodName] = make([]*modelpb.StacktraceFrame, 0)
		}
		typeItem.methods[methodName] = append(typeItem.methods[methodName], frame)

		types[typeName] = typeItem
	}

	return types, nil
}

// Iterates over the classes and methods found in the map file while looking for classes previously found in the stacktrace.
// When it finds a class from the stacktrace, it replaces the obfuscated class name and method names by
// the ones found in the map.
func resolveMappings(types map[string]StacktraceType, mapReader io.Reader) error {
	scanner := bufio.NewScanner(mapReader)
	var currentType *StacktraceType

	for scanner.Scan() {
		line := scanner.Text()
		typeMatch := typePattern.FindStringSubmatch(line)
		if typeMatch != nil {
			// Found a class declaration within the map.
			obfuscatedName := typeMatch[2]
			stacktraceType, ok := types[obfuscatedName]
			if ok {
				// The class found is also in the stacktrace.
				currentType = &stacktraceType
				for _, frames := range stacktraceType.methods {
					for _, frame := range frames {
						if frame.Original == nil {
							frame.Original = &modelpb.Original{}
						}
						// Multiple frames might point to the same class, so we need to deobfuscate the class name for them all.
						frame.Original.Classname = obfuscatedName
						frame.Classname = typeMatch[1]
						frame.SourcemapUpdated = true
					}
				}
			} else {
				// The class found is not part of the stacktrace. We need to clear the current type to avoid looping
				// through this class' methods, as R8 maps list the classes' methods right below the class definition.
				currentType = nil
			}
		} else if currentType != nil {
			// We found a class in the map that is also in the stacktrace, so we enter here to loop through its methods.
			methodMatch := methodPattern.FindStringSubmatch(line)
			if methodMatch != nil {
				// We found a method definition.
				sourceFileStart := methodMatch[1]
				sourceFileEnd := methodMatch[2]
				methodObfuscatedName := methodMatch[4]
				methodRealName := methodMatch[3]
				methodKey := methodObfuscatedName
				if sourceFileStart != "" && sourceFileStart == sourceFileEnd {
					// This method might be compressed, in other words, its deobfuscated form might have multiple lines.
					methodKey += ":" + sourceFileStart
				}
				frames, ok := currentType.methods[methodKey]
				if ok {
					// We found this method in the stacktrace too. Since a method might be referenced multiple times
					// in a single stacktrace, we must make sure to deobfuscate them all.
					for _, frame := range frames {
						if frame.Original.Function == "" {
							frame.Original.Function = methodObfuscatedName
							frame.Function = methodRealName
						} else {
							// If it enters here, it means that this method is compressed and its first line was set
							// previously, so now we have to append extra lines to it.
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
