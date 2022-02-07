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
	"context"
	"errors"
	"flag"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/testcontainers/testcontainers-go/wait"
	"gopkg.in/yaml.v3"

	"github.com/elastic/apm-server/systemtest"
)

const (
	policyDescription = "policy created by apm-server/systemtest/cmd/runapm"
)

var (
	force            bool
	reinstallPackage bool
	keep             bool
	policyName       string
	namespace        string
	vars             = make(varsFlag)
)

func init() {
	flag.StringVar(&policyName, "policy", "runapm", "Agent policy name")
	flag.StringVar(&namespace, "namespace", "default", "Agent policy namespace")
	flag.BoolVar(&force, "f", false, "Force agent policy creation, deleting existing policy if found")
	flag.BoolVar(&reinstallPackage, "reinstall", true, "Reinstall APM integration package")
	flag.BoolVar(&keep, "keep", false, "If true, agent policy and agent will not be destroyed on exit")
	flag.Var(vars, "var", "Define a package var (k=v), with values being YAML-encoded; can be specified more than once")
}

type varsFlag map[string]interface{}

func (f varsFlag) String() string {
	out, err := yaml.Marshal(f)
	if err != nil {
		panic(err)
	}
	return string(out)
}

func (f varsFlag) Set(s string) error {
	i := strings.IndexRune(s, '=')
	if i < 0 {
		return errors.New("missing '='; expected format k=v")
	}
	k, vstr := s[:i], s[i+1:]
	var v interface{}
	if err := yaml.Unmarshal([]byte(vstr), &v); err != nil {
		return err
	}
	f[k] = v
	return nil
}

func Main() error {
	flag.Parse()

	log.Println("Initialising Fleet")
	if err := systemtest.Fleet.Setup(); err != nil {
		log.Fatal(err)
	}
	if force {
		agentPolicies, err := systemtest.Fleet.AgentPolicies("ingest-agent-policies.name:" + policyName)
		if err != nil {
			return err
		}
		if len(agentPolicies) > 0 {
			log.Printf("Deleting existing agent policy")
			agentPolicyIDs := make([]string, len(agentPolicies))
			for i, agentPolicy := range agentPolicies {
				agentPolicyIDs[i] = agentPolicy.ID
			}
			if err := systemtest.DestroyAgentPolicy(agentPolicyIDs...); err != nil {
				return err
			}
		}
	}
	if err := systemtest.InitFleetPackage(reinstallPackage); err != nil {
		return err
	}

	log.Println("Creating Elastic Agent policy")
	agentPolicy, key, err := systemtest.Fleet.CreateAgentPolicy(policyName, namespace, policyDescription)
	if err != nil {
		return err
	}
	packagePolicy := systemtest.NewPackagePolicy(agentPolicy, vars)
	if err := systemtest.Fleet.CreatePackagePolicy(packagePolicy); err != nil {
		return err
	}
	if !keep {
		defer func() {
			log.Println("Destroying agent policy")
			if err := systemtest.DestroyAgentPolicy(agentPolicy.ID); err != nil {
				log.Fatal(err)
			}
		}()
	}

	log.Println("Creating Elastic Agent container")
	agent, err := systemtest.NewUnstartedElasticAgentContainer()
	if err != nil {
		return err
	}
	agent.Reap = !keep
	agent.FleetEnrollmentToken = key.APIKey
	if !keep {
		defer func() {
			log.Println("Terminating agent")
			agent.Close()
		}()
	}

	agent.ExposedPorts = []string{"8200"}
	agent.WaitingFor = wait.ForHTTP("/").WithPort("8200/tcp").WithStartupTimeout(5 * time.Minute)
	agent.Stdout = os.Stdout
	agent.Stderr = os.Stderr
	if err := agent.Start(); err != nil {
		return err
	}

	serverURL := &url.URL{Scheme: "http", Host: agent.Addrs["8200"]}
	log.Printf("Elastic Agent container started")
	log.Printf(" - APM Server listening on %s", serverURL)
	_, err = agent.Wait(context.Background())
	return err
}

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}
