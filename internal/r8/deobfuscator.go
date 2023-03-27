package r8

import (
	"bufio"
	"fmt"
	"os"
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

var symbolPattern = regexp.MustCompile(`^\s*at (.+)\.(.+)\((.+)\)$`)
var sourceFilePattern = regexp.MustCompile(`SourceFile:(\d+)`)
var typePattern = regexp.MustCompile(`^(\S+) -> (\S+):$`)
var methodPattern = regexp.MustCompile(`(?:(\d+):(\d+):)*\S+ (\S+)\(.*\)(?:[:\d]+)* -> (\S+)`)

func Deobfuscate(stacktrace string, mapFilePath string) (string, error) {
	types, err := findUniqueTypes(stacktrace)
	if err != nil {
		return "", err
	}
	mapping, err := findMappingFor(types, mapFilePath)
	if err != nil {
		return "", err
	}

	deobfuscated := stacktrace
	for k, v := range mapping {
		deobfuscated = strings.ReplaceAll(deobfuscated, k, v)
	}

	return deobfuscated, nil
}

func findUniqueTypes(stacktrace string) (map[string]StacktraceType, error) {
	var symbols = make(map[string]StacktraceType)
	scanner := bufio.NewScanner(strings.NewReader(stacktrace))

	for scanner.Scan() {
		line := scanner.Text()
		item := symbolPattern.FindStringSubmatchIndex(line)
		if item != nil {
			typeIndex := item[2]
			typeName := line[typeIndex:item[3]]
			methodName := line[item[4]:item[5]]
			callSite := line[item[6]:item[7]]
			sourceFileMatch := sourceFilePattern.FindStringSubmatch(callSite)
			if sourceFileMatch != nil {
				methodName = fmt.Sprintf("%s:%s", methodName, sourceFileMatch[1])
			}
			symbol, found := symbols[typeName]
			if !found {
				symbol = StacktraceType{typeName, typeIndex + 1, make(map[string]string)}
				symbols[typeName] = symbol
			}
			_, foundMethod := symbol.methods[methodName]
			if !foundMethod {
				symbol.methods[methodName] = callSite
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return symbols, nil
}

func findMappingFor(symbols map[string]StacktraceType, mapFilePath string) (map[string]string, error) {
	mapFile, err := os.Open(mapFilePath)
	if err != nil {
		return nil, err
	}

	defer mapFile.Close()

	var res = make(map[string]string)
	scanner := bufio.NewScanner(mapFile)
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
			stacktraceType, found := symbols[obfuscatedName]
			if found {
				currentType = &MappedType{stacktraceType, typeMatch[1]}
			} else {
				currentType = nil
			}
		} else if currentType != nil {
			methodMatch := methodPattern.FindStringSubmatch(line)
			if methodMatch != nil {
				currentMappedMethodCall = handleMappedMethodCall(res, MethodMatch{
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

func handleMappedMethodCall(res map[string]string, methodMatch MethodMatch, currentType *MappedType, currentMappedMethodCall *MappedMethodCall) *MappedMethodCall {
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
	methodCallSite, foundMethod := currentType.obfuscated.methods[methodNameReference]
	if foundMethod {
		delete(currentType.obfuscated.methods, methodNameReference)
		key := getKey(currentType.obfuscated.name, methodMatch.methodObfuscatedName, methodCallSite)
		res[key] = getKey(currentType.realName, methodMatch.methodRealName, methodCallSite)
		return &MappedMethodCall{mapReference, key}
	} else if currentMappedMethodCall != nil && currentMappedMethodCall.reference == mapReference {
		element, _ := res[currentMappedMethodCall.key]
		res[currentMappedMethodCall.key] = element + "\n" + fmt.Sprintf("%s%s", strings.Repeat(" ", len(currentType.realName)+currentType.obfuscated.indentation), methodMatch.methodRealName)
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
