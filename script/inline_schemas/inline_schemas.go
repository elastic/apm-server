package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
)

const basePath = "./docs/spec/"

func main() {
	schemaPaths := []struct {
		path, schemaOut, goVariable, packageName string
	}{
		{"errors/payload.json", "processor/error/schema.go", "errorSchema", "error"},
		{"transactions/payload.json", "processor/transaction/schema.go", "transactionSchema", "transaction"},
		{"sourcemaps/payload.json", "processor/sourcemap/schema.go", "sourcemapSchema", "sourcemap"},
	}
	for _, schemaInfo := range schemaPaths {
		file := filepath.Join(filepath.Dir(basePath), schemaInfo.path)
		schemaBytes, err := ioutil.ReadFile(file)
		if err != nil {
			panic(err)
		}

		schema, err := replaceRef(filepath.Dir(file), string(schemaBytes))
		if err != nil {
			panic(err)
		}

		publicSchema := fmt.Sprintf("func Schema() string {\n\treturn %s\n}\n", schemaInfo.goVariable)
		goScript := fmt.Sprintf("package %s\n\n%s\nvar %s = `%s`\n", schemaInfo.packageName, publicSchema, schemaInfo.goVariable, schema)
		err = ioutil.WriteFile(schemaInfo.schemaOut, []byte(goScript), 0644)
		if err != nil {
			panic(err)
		}
	}
}

var re = regexp.MustCompile(`\"\$ref\": \"(.*?.json)\"`)
var findAndReplace = map[string]string{}

func replaceRef(currentDir string, schema string) (string, error) {
	matches := re.FindAllStringSubmatch(schema, -1)
	for _, match := range matches {
		pattern := escapePattern(match[0])
		if _, ok := findAndReplace[pattern]; !ok {
			s, err := read(currentDir, match[1])
			if err != nil {
				panic(err)
			}
			findAndReplace[pattern] = trimSchemaPart(s)
		}

		re := regexp.MustCompile(pattern)
		schema = re.ReplaceAllLiteralString(schema, findAndReplace[pattern])
	}
	return schema, nil
}

func read(currentRelativePath string, filePath string) (string, error) {
	path := filepath.Join(currentRelativePath, filePath)
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return replaceRef(filepath.Dir(path), string(file))
}

var reDollar = regexp.MustCompile(`\$`)
var reQuote = regexp.MustCompile(`\"`)

func escapePattern(pattern string) string {
	pattern = reDollar.ReplaceAllLiteralString(pattern, `\$`)
	return reQuote.ReplaceAllLiteralString(pattern, `\"`)
}

func trimSchemaPart(part string) string {
	part = strings.Trim(part, "\n")
	part = strings.Trim(part, "\b")
	part = strings.TrimSuffix(part, "}")
	part = strings.TrimPrefix(part, "{")
	part = strings.Trim(part, "\n")
	return strings.Trim(part, "\b")
}
