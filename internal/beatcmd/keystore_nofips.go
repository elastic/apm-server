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

//go:build !requirefips

package beatcmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/elastic/beats/v7/libbeat/common/cli"
	"github.com/elastic/beats/v7/libbeat/common/terminal"
	"github.com/elastic/elastic-agent-libs/keystore"
)

var keystoreCommand = &cobra.Command{
	Use:   "keystore",
	Short: "Manage secrets keystore",
}

func init() {
	keystoreCommand.AddCommand(genCreateKeystoreCmd())
	keystoreCommand.AddCommand(genAddKeystoreCmd())
	keystoreCommand.AddCommand(genRemoveKeystoreCmd())
	keystoreCommand.AddCommand(genListKeystoreCmd())
}

// keyValueGetter defines a function responsible for fetching
// the key value from the src based on the user option selection.
type keyValueGetter func(key string) ([]byte, error)

func genCreateKeystoreCmd() *cobra.Command {
	var flagForce bool
	command := &cobra.Command{
		Use:   "create",
		Short: "Create keystore",
		Run: cli.RunWith(func(cmd *cobra.Command, args []string) error {
			return createKeystore(flagForce)
		}),
	}
	command.Flags().BoolVar(&flagForce, "force", false, "override the existing keystore")
	return command
}

// addKeystoreCmd wraps the keystore add command and its flags
type addKeystoreCmd struct {
	force   bool
	stdin   bool
	fromDir string
	keys    []string
}

func genAddKeystoreCmd() *cobra.Command {
	c := &addKeystoreCmd{}
	command := &cobra.Command{
		Use:   "add",
		Short: "Add secret",
		Run:   cli.RunWith(c.run),
	}
	command.Flags().BoolVar(&c.stdin, "stdin", false, "Use the stdin as the source of the secret")
	command.Flags().BoolVar(&c.force, "force", false, "Override the existing key")
	command.Flags().StringVar(&c.fromDir, "from-dir", "", "Import every file in the directory as a key (filename) / value (file content) pair")
	return command
}

func (c *addKeystoreCmd) validate(args []string) error {
	if c.fromDir != "" {
		if c.stdin {
			return errors.New("--from-dir cannot be combined with --stdin")
		}
		if len(args) > 0 {
			return errors.New("--from-dir cannot be combined with a key argument")
		}
		if len(c.keys) == 0 {
			return fmt.Errorf("directory %q is empty", c.fromDir)
		}
		return nil
	}

	if len(c.keys) == 0 {
		return errors.New("no key provided")
	}
	if len(c.keys) > 1 {
		return fmt.Errorf("you can only provide one key per invocation, got: %s", c.keys)
	}
	return nil
}

func (c *addKeystoreCmd) run(cmd *cobra.Command, args []string) error {
	c.keys = make([]string, len(args))
	for i, arg := range args {
		c.keys[i] = strings.TrimSpace(arg)
	}
	var err error
	if c.fromDir != "" {
		c.keys, err = getKeysFromDir(c.fromDir)
		if err != nil {
			return fmt.Errorf("failed to create the secret: %w", err)
		}
	}

	if err := c.validate(args); err != nil {
		return fmt.Errorf("failed to create the secret: %w", err)
	}

	store, err := getKeystore()
	if err != nil {
		return fmt.Errorf("failed to create the secret: %w", err)
	}

	var getKeyValue keyValueGetter
	var interactive bool
	switch {
	case c.fromDir != "":
		getKeyValue = getKeyValueFromDir(c.fromDir)
	case c.stdin:
		getKeyValue = getKeyValueFromStdin
	default:
		getKeyValue = getKeyValueFromInput
		interactive = true
	}
	if err := addKey(store, c.keys, getKeyValue, c.force, interactive); err != nil {
		return fmt.Errorf("failed to create the secret: %w", err)
	}
	return nil
}

func genRemoveKeystoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Remove secret",
		Run: cli.RunWith(func(cmd *cobra.Command, args []string) error {
			store, err := getKeystore()
			if err != nil {
				return err
			}
			return removeKey(store, args)
		}),
	}
}

func genListKeystoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List keystore",
		Run: cli.RunWith(func(cmd *cobra.Command, args []string) error {
			store, err := getKeystore()
			if err != nil {
				return err
			}
			return list(store)
		}),
	}
}

func createKeystore(force bool) error {
	store, err := getKeystore()
	if err != nil {
		return err
	}

	writableKeystore, err := keystore.AsWritableKeystore(store)
	if err != nil {
		return fmt.Errorf("error creating the keystore: %w", err)
	}

	if store.IsPersisted() && !force {
		overwrite := terminal.PromptYesNo("A keystore already exists, Overwrite?", false)
		if overwrite {
			err := writableKeystore.Create(true)
			if err != nil {
				return fmt.Errorf("error creating the keystore: %w", err)
			}
		} else {
			fmt.Println("Exiting without creating keystore.")
			return nil
		}
	} else {
		err := writableKeystore.Create(true)
		if err != nil {
			return fmt.Errorf("error creating the keystore: %w", err)
		}
	}
	fmt.Printf("Created apm-server keystore\n")
	return nil
}

func getKeystore() (keystore.Keystore, error) {
	_, _, keystore, _, err := LoadConfig()
	return keystore, err
}

// addKey stores a value for each key, fetched via getKeyValue, and saves the
// keystore once at the end. When interactive is false there is no terminal to
// answer prompts on, so a missing keystore is created and an existing key is
// an error unless force is set.
func addKey(store keystore.Keystore, keys []string, getKeyValue keyValueGetter, force, interactive bool) error {
	writableKeystore, err := keystore.AsWritableKeystore(store)
	if err != nil {
		return fmt.Errorf("error creating the keystore: %w", err)
	}

	if !store.IsPersisted() {
		if !force && interactive {
			create := terminal.PromptYesNo("The keystore does not exist. Do you want to create it?", false)
			if !create {
				return errors.New("exiting without creating keystore")
			}
		}
		err := writableKeystore.Create(true)
		if err != nil {
			return fmt.Errorf("could not create keystore, error: %w", err)
		}
		fmt.Println("Created keystore")
	}

	for _, key := range keys {
		if _, err := store.Retrieve(key); err == nil {
			if !force {
				if !interactive {
					return fmt.Errorf("the settings %s already exist in the keystore use `--force` to replace it", key)
				}
				// Only reachable with a single key, so returning here cannot
				// discard values stored for earlier keys.
				overwrite := terminal.PromptYesNo(fmt.Sprintf("Setting %s already exists, Overwrite?", key), false)
				if !overwrite {
					fmt.Println("Exiting without modifying keystore.")
					return nil
				}
			}
		} else if !errors.Is(err, keystore.ErrKeyDoesntExists) {
			return err
		}

		keyValue, err := getKeyValue(key)
		if err != nil {
			return err
		}

		if err = writableKeystore.Store(key, keyValue); err != nil {
			return fmt.Errorf("could not add the key in the keystore, error: %w", err)
		}

	}

	if err = writableKeystore.Save(); err != nil {
		return fmt.Errorf("fail to save the keystore: %w", err)
	} else {
		fmt.Println("Successfully updated the keystore")
	}
	return nil
}

func removeKey(store keystore.Keystore, keys []string) error {
	if len(keys) == 0 {
		return errors.New("you must supply at least one key to remove")
	}

	writableKeystore, err := keystore.AsWritableKeystore(store)
	if err != nil {
		return fmt.Errorf("error deleting the keystore: %w", err)
	}

	if !store.IsPersisted() {
		return errors.New("the keystore doesn't exist. Use the 'create' command to create one")
	}

	for _, key := range keys {
		_, err := store.Retrieve(key)
		if err != nil {
			return fmt.Errorf("could not find key '%v' in the keystore", key)
		}

		writableKeystore.Delete(key)
		err = writableKeystore.Save()
		if err != nil {
			return fmt.Errorf("could not update the keystore with the changes, key: %s, error: %w", key, err)
		}
		fmt.Printf("successfully removed key: %s\n", key)
	}
	return nil
}

func list(store keystore.Keystore) error {
	listingKeystore, err := keystore.AsListingKeystore(store)
	if err != nil {
		return fmt.Errorf("error listing the keystore: %w", err)
	}
	keys, err := listingKeystore.List()
	if err != nil {
		return fmt.Errorf("could not read values from the keystore, error: %w", err)
	}
	for _, key := range keys {
		fmt.Println(key)
	}
	return nil
}

func getKeyValueFromStdin(_ string) ([]byte, error) {
	reader := bufio.NewReader(os.Stdin)
	keyValue, err := io.ReadAll(reader)
	if err != nil {
		return []byte{}, fmt.Errorf("could not read input from stdin")
	}

	return keyValue, nil
}

func getKeyValueFromInput(key string) ([]byte, error) {
	fmt.Printf("Enter value for %s: ", key)
	keyValue, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return []byte{}, fmt.Errorf("could not read value from the input, error: %w", err)
	}

	return keyValue, nil
}

// getKeyValueFromDir returns a getter that reads the value for key from the
// file of the same name under dir.
func getKeyValueFromDir(dir string) keyValueGetter {
	return func(key string) ([]byte, error) {
		keyValue, err := os.ReadFile(filepath.Join(dir, key))
		if err != nil {
			return nil, fmt.Errorf("could not read value for key %q from %q: %w", key, dir, err)
		}
		return keyValue, nil
	}
}

// getKeysFromDir lists the files directly under dir, skipping subdirectories
// and hidden entries.
func getKeysFromDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("could not read directory %q error: %w", dir, err)
	}

	var keys []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("could not stat %q error: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		keys = append(keys, name)
	}

	return keys, nil
}
