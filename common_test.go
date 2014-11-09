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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/maruel/subcommands/subcommandstest"
)

// makeTempDir creates a temporary directory.
func makeTempDir(t *subcommandstest.TB, name string) string {
	name, err := ioutil.TempDir("", "dumbcas_"+name)
	t.Assertf(err == nil, "Internal error")
	return name
}

func removeTempDir(tempDir string) {
	if err := os.RemoveAll(tempDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to clean up %s", tempDir)
	}
}

func createTree(rootDir string, tree map[string]string) error {
	for relPath, content := range tree {
		base := filepath.Dir(relPath)
		if base != "." {
			if err := os.MkdirAll(filepath.Join(rootDir, base), 0700); err != nil && !os.IsExist(err) {
				return err
			}
		}
		f, err := os.Create(filepath.Join(rootDir, relPath))
		if err != nil {
			return err
		}
		f.WriteString(content)
		f.Sync()
		f.Close()
	}
	return nil
}

// Equals verifies equality of two slices. They must be sorted.
func Equals(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ToSet converts a string slice to a map of string acting as a set.
func ToSet(i []string) map[string]bool {
	out := make(map[string]bool)
	for _, v := range i {
		out[v] = true
	}
	return out
}

// Sub substracts list b from list a; e.g. it retuns "a-b".
// It keeps the a slice ordered.
func Sub(a []string, b []string) []string {
	bMap := ToSet(b)
	out := []string{}
	for _, v := range a {
		if !bMap[v] {
			out = append(out, v)
		}
	}
	return out
}

func sha1String(content string) string {
	hash := sha1.New()
	io.WriteString(hash, content)
	return hex.EncodeToString(hash.Sum(nil))
}
