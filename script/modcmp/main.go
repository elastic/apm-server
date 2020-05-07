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

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

func readGoMod(modpath string) (*modfile.File, error) {
	args := []string{"list", "-m", "-f={{.GoMod}}"}
	if modpath != "" {
		args = append(args, modpath)
	}
	stdout, err := exec.Command("go", args...).Output()
	if err != nil {
		return nil, err
	}
	gomodPath := strings.TrimSpace(string(stdout))
	data, err := ioutil.ReadFile(gomodPath)
	if err != nil {
		return nil, err
	}
	return modfile.Parse(gomodPath, data, nil)
}

func goModWhy(modpaths ...string) (map[string][]string, error) {
	args := append([]string{"mod", "why", "-m"}, modpaths...)
	stdout, err := exec.Command("go", args...).Output()
	if err != nil {
		return nil, err
	}

	var modpath string
	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	required := make(map[string][]string)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			modpath = ""
			continue
		} else if strings.HasPrefix(line, "# ") {
			modpath = strings.TrimPrefix(line, "# ")
			continue
		}
		if modpath != "" && !strings.HasPrefix(line, "(main module does not need module") {
			required[modpath] = append(required[modpath], line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return required, nil
}

// Main compares the dependencies of the module with path
// modpath2 (e.g. apm-server), against those of the module
// with path modpath1 (e.g. beats).
//
// Main will report the following issues:
//  - modpath2 has a dependency older than the one specified in modpath1
//  - modpath2 is missing a "replace" clause specified in modpath1,
//    unless that module is not required by modpath2
//  - modpath2 is missing an "exclude" clause specified in modpath1,
//    unless that module is not required by modpath2
func Main(args []string) error {
	modpath1 := args[0]
	var modpath2 string
	if len(args) == 2 {
		modpath2 = args[1]
	}
	gomod1, err := readGoMod(modpath1)
	if err != nil {
		return err
	}
	gomod2, err := readGoMod(modpath2)
	if err != nil {
		return err
	}
	if modpath2 == "" {
		modpath2 = gomod2.Module.Mod.Path
	}

	// Compare Require: make sure all modules in gomod2 have an equal or greater
	// version compared to the one in gomod1.
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 1, ' ', 0)
	var writtenHeading bool
	var discrepancies int
	gomod1Requires := make(map[string]*modfile.Require)
	for _, require := range gomod1.Require {
		gomod1Requires[require.Mod.Path] = require
	}
	for _, require := range gomod2.Require {
		gomod1Require, ok := gomod1Requires[require.Mod.Path]
		if !ok {
			continue
		}
		if semver.Compare(require.Mod.Version, gomod1Require.Mod.Version) < 0 {
			if !writtenHeading {
				fmt.Fprintf(w, "Require\t%s Version\t%s Version\n", modpath1, modpath2)
				writtenHeading = true
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", require.Mod.Path, gomod1Require.Mod.Version, require.Mod.Version)
			discrepancies++
		}
	}
	w.Flush()

	// Check which of the Exclude and Replace modules in gomod1
	// are required by gomod2.
	var checkRequired []string
	for _, exclude := range gomod1.Exclude {
		checkRequired = append(checkRequired, exclude.Mod.Path)
	}
	for _, replace := range gomod1.Replace {
		checkRequired = append(checkRequired, replace.Old.Path)
	}
	gomod2Required, err := goModWhy(checkRequired...)
	if err != nil {
		return err
	}

	// Compare Exclude: make sure all excludes in gomod1 are present in gomod2.
	w.Init(os.Stdout, 0, 4, 1, ' ', 0)
	writtenHeading = false
	gomod2Excludes := make(map[module.Version]bool)
	for _, exclude := range gomod2.Exclude {
		gomod2Excludes[exclude.Mod] = true
	}
	for _, exclude := range gomod1.Exclude {
		if gomod2Excludes[exclude.Mod] || len(gomod2Required[exclude.Mod.Path]) == 0 {
			continue
		}
		if !writtenHeading {
			fmt.Fprintf(w, "Exclude\t%s\t%s\n", modpath1, modpath2)
			writtenHeading = true
		}
		fmt.Fprintf(w, "%s\t%s\tmissing\n", exclude.Mod.Path, exclude.Mod.Version)
		discrepancies++
	}
	w.Flush()

	// Compare Replace: make sure all replacements in gomod1 are present and the
	// same in gomod2.
	w.Init(os.Stdout, 0, 4, 1, ' ', 0)
	writtenHeading = false
	gomod2Replaces := make(map[module.Version]*modfile.Replace)
	for _, replace := range gomod2.Replace {
		gomod2Replaces[replace.Old] = replace
	}
	for _, replace := range gomod1.Replace {
		if len(gomod2Required[replace.Old.Path]) == 0 {
			continue
		}
		gomod2Replace, ok := gomod2Replaces[replace.Old]
		if ok && replace.New == gomod2Replace.New {
			continue
		}
		if !writtenHeading {
			fmt.Fprintf(w, "Replace\t%s\t%s\n", modpath1, modpath2)
			writtenHeading = true
		}
		fmt.Fprintf(w, "%s\t%s\t", replace.Old, replace.New)
		if ok {
			fmt.Fprintln(w, gomod2Replace.New)
		} else {
			fmt.Fprintln(w, "missing")
		}
		discrepancies++
	}
	w.Flush()

	if discrepancies > 0 {
		word := "discrepancy"
		if discrepancies != 1 {
			word = "discrepancies"
		}
		return fmt.Errorf("found %d %s", discrepancies, word)
	}
	return nil
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s <module1> [<module2>]\n", os.Args[0])
		os.Exit(1)
	}
	if err := Main(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
