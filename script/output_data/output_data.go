package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/elastic/apm-server/beater"
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

	var checked = map[string]struct{}{}

	for path, mapping := range beater.Routes {

		if path == beater.HealthCheckURL {
			continue
		}

		p := mapping.ProcessorFactory(httptest.NewRequest("GET", "/", nil))

		// Remove version from name and and s at the end
		name := p.Name()

		// Create path to test docs
		path := filepath.Join(basepath, name, filename)

		f, err := os.Open(path)
		if err != nil {
			return err
		}

		data, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		err = p.Validate(data)
		if err != nil {
			return err
		}

		events, err := p.Transform(data)

		if err != nil {
			return err
		}

		for _, d := range events {
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
				if err != nil {
					return err
				}
				err = ioutil.WriteFile(file, output, 0644)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
