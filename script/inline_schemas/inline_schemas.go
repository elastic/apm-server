package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const basePath = "./docs/spec/"

func main() {
	schemaPaths := []struct {
		path, schemaOut, goVariable string
	}{
		{"errors/payload.json", "processor/error/generated-schemas/payload.go", "PayloadSchema"},
		{"transactions/payload.json", "processor/transaction/generated-schemas/payload.go", "PayloadSchema"},
		{"sourcemaps/payload.json", "processor/sourcemap/generated-schemas/payload.go", "SourcemapSchema"},

		{"transactions/transaction.json", "processor/transaction/generated-schemas/transaction.go", "TransactionSchema"},
		{"transactions/span.json", "processor/transaction/generated-schemas/span.go", "SpanSchema"},
		{"errors/error.json", "processor/error/generated-schemas/error.go", "ErrorSchema"},
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

		goScript := fmt.Sprintf("package schemas\n\nconst %s = `%s`\n", schemaInfo.goVariable, schema)
		err = os.MkdirAll(path.Dir(schemaInfo.schemaOut), os.ModePerm)
		if err != nil {
			panic(err)
		}
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
