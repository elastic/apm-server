package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/elastic/apm-server/beater"
	"github.com/elastic/apm-server/tests"
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
	basePath := "data/valid"
	outputPath := "docs/data/elasticsearch/"

	var checked = map[string]struct{}{}

	for path, mapping := range beater.Routes {

		if path == beater.HealthCheckURL {
			continue
		}

		p := mapping.ProcessorFactory(nil)

		data, err := tests.LoadData(filepath.Join(basePath, p.Name(), filename), p.Name())
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
