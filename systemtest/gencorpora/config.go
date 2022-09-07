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

package gencorpora

import (
	"flag"
	"fmt"
	"path/filepath"

	"go.uber.org/zap/zapcore"
)

const (
	defaultDir        = "./"
	defaultFilePrefix = "es_corpora"
)

var gencorporaConfig = struct {
	CorporaPath               string
	MetadataPath              string
	LoggingLevel              zapcore.Level
	ReplayCount               int
	OverrideMetaSourceRootDir string
}{
	CorporaPath:  filepath.Join(defaultDir, getCorporaPath(defaultFilePrefix)),
	MetadataPath: filepath.Join(defaultDir, getMetaPath(defaultFilePrefix)),
	LoggingLevel: zapcore.WarnLevel,
}

func init() {
	filePrefix := flag.String(
		"file-prefix",
		defaultFilePrefix,
		"Prefix for the generated corpora document and metadata file",
	)
	flag.Func(
		"write-dir",
		"Directory for writing the generated ES corpora and metadata, uses current dir if empty",
		func(writeDir string) error {
			gencorporaConfig.CorporaPath = filepath.Join(writeDir, getCorporaPath(*filePrefix))
			gencorporaConfig.MetadataPath = filepath.Join(writeDir, getMetaPath(*filePrefix))
			return nil
		},
	)
	flag.IntVar(
		&gencorporaConfig.ReplayCount,
		"replay-count",
		1,
		"Number of times the events are replayed",
	)
	flag.Var(
		&gencorporaConfig.LoggingLevel,
		"logging-level",
		"Logging level for APM Server",
	)
	flag.StringVar(
		&gencorporaConfig.OverrideMetaSourceRootDir,
		"override-meta-source-rootdir",
		"",
		"Override the root dir used in the source field in generated metadata with the provided string",
	)
}

func getCorporaPath(prefix string) string {
	return fmt.Sprintf("%s_docs.ndjson", prefix)
}

func getMetaPath(prefix string) string {
	return fmt.Sprintf("%s_meta.json", prefix)
}
