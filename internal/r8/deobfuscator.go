package r8

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type StacktraceType struct {
	name        string
	indentation int
	methods     map[string]string // Maps method references to their stacktrace call site
}

type MappedType struct {
	obfuscated StacktraceType
	realName   string
}

type MappedMethodCall struct {
	reference string
	key       string
}

type MethodMatch struct {
	sourceFileStart      string
	sourceFileEnd        string
	methodRealName       string
	methodObfuscatedName string
}

var (
	symbolPattern     = regexp.MustCompile(`^\s*at (.+)\.(.+)\((.+)\)$`)
	sourceFilePattern = regexp.MustCompile(`SourceFile:(\d+)`)
	typePattern       = regexp.MustCompile(`^(\S+) -> (\S+):$`)
	methodPattern     = regexp.MustCompile(`(?:(\d+):(\d+):)*\S+ (\S+)\(.*\)(?:[:\d]+)* -> (\S+)`)
)

// Deobfuscate parses the stacktrace looking for type names and their methods, then searches for those stacktrace items through the mapFile, looking
// for their de-obfuscated names to later replace the ones in the original stacktrace by their real names found within the mapFile.
func Deobfuscate(stacktrace string, mapFile io.Reader) (string, error) {
	types, err := findUniqueTypes(stacktrace)
	if err != nil {
		return "", err
	}
	mapping, err := findMappingFor(types, mapFile)
	if err != nil {
		return "", err
	}

	deobfuscated := stacktrace
	for k, v := range mapping {
		// Uses ReplaceAll since an obfuscated name may be present several times in a single stacktrace.
		deobfuscated = strings.ReplaceAll(deobfuscated, k, v)
	}

	return deobfuscated, nil
}

func findUniqueTypes(stacktrace string) (map[string]StacktraceType, error) {
	var symbols = make(map[string]StacktraceType)
	scanner := bufio.NewScanner(strings.NewReader(stacktrace))

	for scanner.Scan() {
		line := scanner.Text()
		indices := symbolPattern.FindStringSubmatchIndex(line)
		if indices != nil {
			typeIndex := indices[2]
			typeName := line[typeIndex:indices[3]]
			methodName := line[indices[4]:indices[5]]
			callSite := line[indices[6]:indices[7]]
			sourceFileMatch := sourceFilePattern.FindStringSubmatch(callSite)
			if sourceFileMatch != nil {
				methodName = fmt.Sprintf("%s:%s", methodName, sourceFileMatch[1])
			}
			symbol, ok := symbols[typeName]
			if !ok {
				symbol = StacktraceType{typeName, typeIndex + 1, make(map[string]string)}
				symbols[typeName] = symbol
			}
			_, ok = symbol.methods[methodName]
			if !ok {
				symbol.methods[methodName] = callSite
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return symbols, nil
}

func findMappingFor(symbols map[string]StacktraceType, mapReader io.Reader) (map[string]string, error) {
	var res = make(map[string]string)
	scanner := bufio.NewScanner(mapReader)
	var currentType *MappedType
	var currentMappedMethodCall *MappedMethodCall

	for scanner.Scan() {
		line := scanner.Text()
		typeMatch := typePattern.FindStringSubmatch(line)
		if typeMatch != nil {
			currentMappedMethodCall = nil
			if currentType != nil {
				ensureAllMethodsProvided(res, currentType)
			}
			obfuscatedName := typeMatch[2]
			stacktraceType, ok := symbols[obfuscatedName]
			if ok {
				currentType = &MappedType{stacktraceType, typeMatch[1]}
			} else {
				currentType = nil
			}
		} else if currentType != nil {
			methodMatch := methodPattern.FindStringSubmatch(line)
			if methodMatch != nil {
				currentMappedMethodCall = upsertMappedMethodCall(res, MethodMatch{
					methodMatch[1],
					methodMatch[2],
					methodMatch[3],
					methodMatch[4],
				}, currentType, currentMappedMethodCall)
			}
		}
	}
	if currentType != nil {
		ensureAllMethodsProvided(res, currentType)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return res, nil
}

// Checks if the found methodMatch from the map file is part of the methods previously parsed for the currentType within the
// stacktrace.
// If the method is found, then it's added to the mapping in order to be used for replacing the obfuscated name later, it also returns it as the current MappedMethodCall.
// If the method is NOT found, but it turns out to be a continuation of a previously found method (currentMappedMethodCall), then it appends it to the existing method replacement in the mapping.
// If the method is NOT found and is also NOT a continuation for the currentMappedMethodCall, then the methodMatch is ignored.
func upsertMappedMethodCall(mapping map[string]string, methodMatch MethodMatch, currentType *MappedType, currentMappedMethodCall *MappedMethodCall) *MappedMethodCall {
	methodNameReference := methodMatch.methodObfuscatedName
	if methodMatch.sourceFileStart != "" {
		if methodMatch.sourceFileStart != methodMatch.sourceFileEnd {
			// This is probably due an edge-case where the mapping line starts with different numbers (e.g 1:2). We don't
			// have that case in our tests, therefore we are ignoring it.
			return currentMappedMethodCall
		}
		methodNameReference = fmt.Sprintf("%s:%s", methodMatch.methodObfuscatedName, methodMatch.sourceFileStart)
	}
	mapReference := currentType.obfuscated.name + ":" + methodNameReference
	methodCallSite, ok := currentType.obfuscated.methods[methodNameReference]
	if ok {
		// Found this method in the list of methods parsed from the stacktrace for the currentType.
		delete(currentType.obfuscated.methods, methodNameReference)
		key := getKey(currentType.obfuscated.name, methodMatch.methodObfuscatedName, methodCallSite)
		mapping[key] = getKey(currentType.realName, methodMatch.methodRealName, methodCallSite)
		return &MappedMethodCall{mapReference, key}
	} else if currentMappedMethodCall != nil && currentMappedMethodCall.reference == mapReference {
		// This is a continuation call for the currentMappedMethodCall.
		mapping[currentMappedMethodCall.key] += "\n" + fmt.Sprintf("%s%s", strings.Repeat(" ", len(currentType.realName)+currentType.obfuscated.indentation), methodMatch.methodRealName)
	}

	return currentMappedMethodCall
}

func ensureAllMethodsProvided(res map[string]string, currentType *MappedType) {
	for methodName, callSite := range currentType.obfuscated.methods {
		key := getKey(currentType.obfuscated.name, methodName, callSite)
		res[key] = getKey(currentType.realName, methodName, callSite)
	}
}

func getKey(typeName string, methodName string, callSite string) string {
	return fmt.Sprintf("%s.%s(%s)", typeName, methodName, callSite)
}
