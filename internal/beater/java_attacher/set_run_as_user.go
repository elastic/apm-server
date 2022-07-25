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

//go:build !windows
// +build !windows

package javaattacher

import (
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

func (j *JavaAttacher) setRunAsUser(jvm *jvmDetails, cmd *exec.Cmd) error {
	currentUser, err := user.Current()
	if err != nil {
		// If we cannot get the current user, then we assume apm-server
		// is not the same as the target JVM user.
		j.logger.Warnf("failed to get the current user: %v", err)
	} else {
		j.logger.Debugf("current user: %v", currentUser)
	}

	if currentUser.Gid != jvm.gid || currentUser.Uid != jvm.uid {
		uid, err := strconv.ParseInt(jvm.uid, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid UID %q: %w", jvm.uid, err)
		}
		gid, err := strconv.ParseInt(jvm.gid, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid GID %q: %w", jvm.gid, err)
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	}
	return nil
}
