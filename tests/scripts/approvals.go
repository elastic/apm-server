package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yudai/gojsondiff/formatter"

	"github.com/elastic/apm-server/tests"
)

func main() {
	os.Exit(approval())
}

func approval() int {
	cwd, _ := os.Getwd()
	receivedFiles := findFiles(cwd, tests.ReceivedSuffix)

	for _, rf := range receivedFiles {
		path := strings.Replace(rf, tests.ReceivedSuffix, "", 1)
		_, approved, d, err := tests.Compare(path, map[string]string{})

		if err != nil {
			fmt.Println("Could not create diff ", err)
			return 3
		}

		var aJson map[string]interface{}
		json.Unmarshal(approved, &aJson)
		config := formatter.AsciiFormatterConfig{
			ShowArrayIndex: true,
			Coloring:       true,
		}
		formatter := formatter.NewAsciiFormatter(aJson, config)
		diffString, _ := formatter.Format(d)

		fmt.Println(diffString)
		fmt.Println(rf)
		fmt.Println("\nApprove Changes? (y/n)")
		reader := bufio.NewReader(os.Stdin)
		input, _, _ := reader.ReadRune()
		switch input {
		case 'y':
			approvedPath := strings.Replace(rf, tests.ReceivedSuffix, tests.ApprovedSuffix, 1)
			os.Rename(rf, approvedPath)
		}
	}
	return 0
}

func findFiles(rootDir string, suffix string) []string {
	files := []string{}
	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, suffix) {
			files = append(files, path)
		}
		return nil
	})
	return files
}
