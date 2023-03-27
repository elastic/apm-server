package r8

import (
	"bufio"
	"fmt"
	"log"
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

func Deobfuscate(stacktrace string, mapFilePath string) string {
	types := findUniqueTypes(stacktrace)
	mapping := findMappingFor(types, mapFilePath)

	deobfuscated := stacktrace
	for k, v := range mapping {
		deobfuscated = strings.ReplaceAll(deobfuscated, k, v)
	}

	return deobfuscated
}

func findUniqueTypes(stacktrace string) []StacktraceType {
	symbolPattern := regexp.MustCompile("^\\s*at (.+)(?:\\.(.+)\\((.+)\\))$")
	sourceFilePattern := regexp.MustCompile("SourceFile:(\\d+)")

	var symbols = make([]*StacktraceType, 0)
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
			symbol := getSymbolForType(typeName, symbols)
			if symbol == nil {
				symbol = &StacktraceType{typeName, typeIndex + 1, make(map[string]string)}
				symbols = append(symbols, symbol)
			}
			_, foundMethod := symbol.methods[methodName]
			if !foundMethod {
				symbol.methods[methodName] = callSite
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	var result = make([]StacktraceType, len(symbols))
	for i, v := range symbols {
		result[i] = *v
	}

	return result
}

func getSymbolForType(typeName string, symbols []*StacktraceType) *StacktraceType {
	for _, v := range symbols {
		if v.name == typeName {
			return v
		}
	}
	return nil
}

func findMappingFor(symbols []StacktraceType, mapFilePath string) map[string]string {
	mapFile, err := os.Open(mapFilePath)
	if err != nil {
		log.Fatal(err)
	}

	defer mapFile.Close()

	var res = make(map[string]string)
	typePattern := regexp.MustCompile("^([^\\s]+) -> ([^\\s]+):$")
	methodPattern := regexp.MustCompile("(?:(\\d+):(\\d+):)*[^\\s]+ ([^\\s]+)\\(.*\\)(?:[:\\d]+)* -> ([^\\s]+)")
	scanner := bufio.NewScanner(mapFile)
	var currentType *MappedType
	var currentMappedMethodCall *MappedMethodCall

	for scanner.Scan() {
		line := scanner.Text()
		typeMatch := typePattern.FindStringSubmatch(line)
		if typeMatch != nil {
			currentMappedMethodCall = nil
			if currentType != nil {
				ensureAllMethodsProvided(&res, currentType)
			}
			obfuscatedName := typeMatch[2]
			stacktraceType := getStacktraceType(symbols, obfuscatedName)
			if stacktraceType != nil {
				currentType = &MappedType{*stacktraceType, typeMatch[1]}
			} else {
				currentType = nil
			}
		} else if currentType != nil {
			methodMatch := methodPattern.FindStringSubmatch(line)
			if methodMatch != nil {
				currentMappedMethodCall = handleMappedMethodCall(&res, methodMatch, currentType, currentMappedMethodCall)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return res
}

func handleMappedMethodCall(res *map[string]string, methodMatch []string, currentType *MappedType, currentMappedMethodCall *MappedMethodCall) *MappedMethodCall {
	sourceFileStart := methodMatch[1]
	sourceFileEnd := methodMatch[2]
	methodRealName := methodMatch[3]
	methodObfuscatedName := methodMatch[4]
	methodNameReference := methodObfuscatedName
	if sourceFileStart != "" {
		if sourceFileStart != sourceFileEnd {
			// This is probably due an edge-case where the mapping line starts with different numbers (e.g 1:2). We don't
			// have that case in our tests, therefore we are ignoring it.
			return currentMappedMethodCall
		}
		methodNameReference = fmt.Sprintf("%s:%s", methodObfuscatedName, sourceFileStart)
	}
	mapReference := currentType.obfuscated.name + ":" + methodNameReference
	methodCallSite, foundMethod := currentType.obfuscated.methods[methodNameReference]
	if foundMethod {
		delete(currentType.obfuscated.methods, methodNameReference)
		key := getKey(currentType, methodObfuscatedName, methodCallSite)
		addMainCall(res, key, currentType, methodRealName, methodCallSite)
		return &MappedMethodCall{mapReference, key}
	} else if currentMappedMethodCall != nil && currentMappedMethodCall.reference == mapReference {
		element, _ := (*res)[currentMappedMethodCall.key]
		(*res)[currentMappedMethodCall.key] = element + "\n" + fmt.Sprintf("%s%s", strings.Repeat(" ", len(currentType.realName)+currentType.obfuscated.indentation), methodRealName)
	}

	return currentMappedMethodCall
}

func ensureAllMethodsProvided(res *map[string]string, currentType *MappedType) {
	for methodName, callSite := range currentType.obfuscated.methods {
		key := getKey(currentType, methodName, callSite)
		addMainCall(res, key, currentType, methodName, callSite)
	}
}

func getKey(currentType *MappedType, methodName string, callSite string) string {
	return fmt.Sprintf("%s.%s(%s)", currentType.obfuscated.name, methodName, callSite)
}

func addMainCall(res *map[string]string, key string, currentType *MappedType, methodRealName string, callSite string) {
	(*res)[key] = fmt.Sprintf("%s.%s(%s)", currentType.realName, methodRealName, callSite)
}

func getStacktraceType(symbols []StacktraceType, typeName string) *StacktraceType {
	for _, v := range symbols {
		if v.name == typeName {
			return &v
		}
	}

	return nil
}
