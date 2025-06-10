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

package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-exec/tfexec"
)

// Var is a helper to simplify creating Terraform vars to pass to terraform-exec.
func Var(name, value string) *tfexec.VarOption {
	return tfexec.Var(fmt.Sprintf("%s=%s", name, value))
}

type Runner struct {
	initialized bool
	outputs     map[string]tfexec.OutputMeta
	tf          *tfexec.Terraform
}

func NewRunner(t *testing.T, workingDir string) (*Runner, error) {
	tr := Runner{}

	tf, err := tfexec.NewTerraform(workingDir, "terraform")
	if err != nil {
		return &tr, fmt.Errorf("error instantiating terraform runner: %w", err)
	}
	tf.SetLogger(&tfLoggerv2{t})
	tr.tf = tf
	if err := tr.init(); err != nil {
		return &tr, fmt.Errorf("cannot run terraform init: %w", err)
	} else {
		tr.initialized = true
	}

	return &tr, nil
}

func (t *Runner) init() error {
	return t.tf.Init(context.Background(), tfexec.Upgrade(true))
}

func (t *Runner) Apply(ctx context.Context, vars ...tfexec.ApplyOption) error {
	if !t.initialized {
		if err := t.init(); err != nil {
			return fmt.Errorf("cannot init before apply: %w", err)
		}
	}
	if err := t.tf.Apply(ctx, vars...); err != nil {
		return fmt.Errorf("cannot apply: %w", err)
	}

	output, err := t.tf.Output(ctx)
	if err != nil {
		return fmt.Errorf("cannot run terraform output: %w", err)
	}

	t.outputs = output
	return nil
}

func (t *Runner) Destroy(ctx context.Context, vars ...tfexec.DestroyOption) error {
	if !t.initialized {
		if err := t.init(); err != nil {
			return fmt.Errorf("cannot init before apply: %w", err)
		}
	}

	return t.tf.Destroy(ctx, vars...)
}

func (t *Runner) Output(name string, res any) error {
	o, ok := t.outputs[name]
	if !ok {
		return fmt.Errorf("output named %s not found", name)
	}
	if err := json.Unmarshal(o.Value, res); err != nil {
		return fmt.Errorf("cannot unmarshal output: %w", err)
	}
	return nil
}

// tfLoggerv2 wraps a testing.TB to implement the printfer interface
// required by terraform-exec logger.
type tfLoggerv2 struct {
	testing.TB
}

// Printf implements terraform-exec.printfer interface
func (l *tfLoggerv2) Printf(format string, v ...interface{}) {
	l.Logf(format, v...)
}
