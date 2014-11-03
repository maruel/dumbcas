/* Copyright 2012 Marc-Antoine Ruel. Licensed under the Apache License, Version
2.0 (the "License"); you may not use this file except in compliance with the
License.  You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0. Unless required by applicable law or
agreed to in writing, software distributed under the License is distributed on
an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
or implied. See the License for the specific language governing permissions and
limitations under the License. */

package main

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/maruel/dumbcas/dumbcaslib"
	"github.com/maruel/subcommands"
)

// CommonFlags is common flags for all commands.
type CommonFlags struct {
	subcommands.CommandRunBase
	Root string
	// These are not "flags" per se but are created indirectly by the -root flag.
	cas   dumbcaslib.CasTable
	nodes dumbcaslib.NodesTable
}

// Init initializes the common flags.
func (c *CommonFlags) Init() {
	c.Flags.StringVar(&c.Root, "root", os.Getenv("DUMBCAS_ROOT"), "Root directory; required. Set $DUMBCAS_ROOT to set a default.")
}

// Parse parses the common flags.
func (c *CommonFlags) Parse(d DumbcasApplication, bypassFsck bool) error {
	if c.Root == "" {
		return errors.New("Must provide -root")
	}
	root, err := filepath.Abs(c.Root)
	if err != nil {
		return fmt.Errorf("Failed to find %s", c.Root)
	}
	c.Root = root

	cas, err := d.MakeCasTable(c.Root)
	if err != nil {
		return err
	}
	c.cas = cas

	if c.cas.GetFsckBit() {
		if !bypassFsck {
			return fmt.Errorf("Can't run if fsck is needed. Please run fsck first.")
		}
		fmt.Fprintf(os.Stderr, "WARNING: fsck is needed.")
	}
	nodes, err := d.LoadNodesTable(c.Root, c.cas)
	if err != nil {
		return err
	}
	c.nodes = nodes
	return nil
}

func sha1Reader(f io.Reader) (string, error) {
	hash := sha1.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func sha1File(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = f.Close()
	}()
	return sha1Reader(f)
}
