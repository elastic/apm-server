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
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
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
		uid, gid, err := parseUserIds(jvm.uid, jvm.gid)
		if err != nil {
			return err
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	}
	return nil
}

// getAttacherJar finds an attacher jar based on the given uid.
// In POSIX-compliant systems, it would be an attacher jar owned by the given user and with 0600 access mode.
// If such was not yet created, this function calls createAttacherTempDir to create it and keep a mapping for it
func (j *JavaAttacher) getAttacherJar(uid, gid string) string {
	attacherJar := j.uidToAttacherJar[uid]
	if attacherJar == "" {
		tmpAttacherJar, err := j.createAttacherTempDir(uid, gid)
		if err != nil {
			j.logger.Errorf("failed to create tmp dir for user %v, using the bundled attacher jar", err)
			// this is required so that we don't try to create tmp dir and copy every time we see a JVM ran by this user
			attacherJar = bundledJavaAttacher
		} else {
			attacherJar = tmpAttacherJar
		}
		j.uidToAttacherJar[uid] = attacherJar
	}
	return attacherJar
}

// createAttacherTempDir looks for a temp dir that is already mapped to the given user.
// If such is not found, creates one as follows:
// 	1.	create a temporary dir with 0700 access
//	2.	copy the bundled attacher jar into the tmp dir
//	3.	change the jar access mode to 0600
//	4.	change tmp dir and tmp attacher jar owner to the given user
//	5.	keep a mapping of this jar to the user ID
func (j *JavaAttacher) createAttacherTempDir(uidS, gidS string) (string, error) {
	// creates the temp dir with access mode 0700
	tempDir, err := os.MkdirTemp("", "elasticapmagent-*")
	if err != nil {
		return "", err
	}
	// keep track so we eventually delete
	j.tmpDirs = append(j.tmpDirs, tempDir)
	uid, gid, err := parseUserIds(uidS, gidS)
	if err != nil {
		return "", err
	}
	bundledAttacherFile, err := os.Open(bundledJavaAttacher)
	if err != nil {
		return "", fmt.Errorf("failed to open bundled attacher jar: %w", err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer bundledAttacherFile.Close()
	tmpAttacherJarPath := filepath.Join(tempDir, bundledJavaAttacher)
	tmpAttacherJarFile, err := os.Create(tmpAttacherJarPath)
	if err != nil {
		return "", fmt.Errorf("failed to create tmp attacher jar: %w", err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer tmpAttacherJarFile.Close()
	nBytes, err := io.Copy(tmpAttacherJarFile, bundledAttacherFile)
	if err != nil {
		return "", fmt.Errorf("failed to copy bundled attacher jar to %v: %w", bundledJavaAttacher, err)
	}
	j.logger.Debugf("%v (%v bytes) successfully copied to %v", bundledJavaAttacher, nBytes, tmpAttacherJarPath)
	err = os.Chmod(tmpAttacherJarPath, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to change permissions of %v to 0600: %w", tmpAttacherJarPath, err)
	}
	err = os.Chown(tempDir, int(uid), int(gid))
	if err != nil {
		return "", fmt.Errorf("failed to change owner of %v to be %v: %w", tempDir, uidS, err)
	}
	err = os.Chown(tmpAttacherJarPath, int(uid), int(gid))
	if err != nil {
		return "", fmt.Errorf("failed to change owner of %v to be %v: %w", tmpAttacherJarPath, uidS, err)
	}
	return tmpAttacherJarPath, nil
}

func parseUserIds(uidS, gidS string) (int, int, error) {
	uid, err := strconv.Atoi(uidS)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid UID %q: %w", uidS, err)
	}
	gid, err := strconv.Atoi(gidS)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid GID %q: %w", gidS, err)
	}
	return uid, gid, nil
}

func (j *JavaAttacher) cleanResources() {
	for _, dir := range j.tmpDirs {
		// todo delete all contents recursively in case there is still a jar held by the another process
		err := os.RemoveAll(dir)
		if err != nil {
			j.logger.Errorf("failed to delete tmp dir %v: %v", dir, err)
		}
	}
}
