package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	_ "github.com/elastic/apm-server/include"
	"github.com/elastic/apm-server/processor"
)

func main() {

	err := generate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}

func generate() error {
	filename := "payload.json"
	basepath := "tests/data/valid"
	outputPath := "docs/data/elasticsearch/"

	processors := processor.Registry.GetProcessors()

	var checked = map[string]struct{}{}

	for _, p := range processors {

		// Remove version from name and and s at the end
		name := p.Name()

		// Create path to test docs
		path := filepath.Join(basepath, name, filename)

		f, err := os.Open(path)
		if err != nil {
			return err
		}

		err = p.Validate(f)
		if err != nil {
			return err
		}

		data := p.Transform()

		for _, d := range data {
			n, err := d.GetValue("processor.name")
			if err != nil {
				return err
			}
			event, err := d.GetValue("processor.event")
			if err != nil {
				return err
			}

			key := n.(string) + "-" + event.(string)

			if _, ok := checked[key]; !ok {
				checked[key] = struct{}{}
				file := filepath.Join(outputPath, event.(string)+".json")

				output, err := json.MarshalIndent(d.Fields, "", "    ")
				err = ioutil.WriteFile(file, output, 0644)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
