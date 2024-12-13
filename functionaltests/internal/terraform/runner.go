package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-exec/tfexec"
)

type Runner struct {
	initialized bool
	outputs     map[string]tfexec.OutputMeta
	tf          *tfexec.Terraform
}

func New(t *testing.T, workingDir string) (*Runner, error) {
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

func (t *Runner) Output(name string, res any) error {
	o := t.outputs[name]
	if err := json.Unmarshal(o.Value, res); err != nil {
		return fmt.Errorf("cannot unmarshal output: %w", err)
	}
	return nil
}
