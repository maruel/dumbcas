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
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"os"
	"path"
)

func GetRandRune() rune {
	chars := "0123456789abcdefghijklmnopqrstuvwxyz"
	lengthBig := big.NewInt(int64(len(chars)))
	val, err := rand.Int(rand.Reader, lengthBig)
	if err != nil {
		panic("Rand failed")
	}
	return rune(chars[int(val.Int64())])
}

// Creates a temporary directory.
func makeTempDir(t *TB, name string) string {
	prefix := "dumbcas_" + name + "_"
	length := 8
	tempDir := os.TempDir()

	ranPath := make([]rune, length)
	for i := 0; i < length; i++ {
		ranPath[i] = GetRandRune()
	}
	tempFull := path.Join(tempDir, prefix+string(ranPath))
	for {
		err := os.Mkdir(tempFull, 0700)
		if os.IsExist(err) {
			// Add another random character.
			ranPath = append(ranPath, GetRandRune())
		}
		return tempFull
	}
	t.Assertf(false, "Internal error")
	return ""
}

func removeTempDir(tempDir string) {
	if err := os.RemoveAll(tempDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to clean up %s", tempDir)
	}
}

func createTree(rootDir string, tree map[string]string) error {
	for relPath, content := range tree {
		base := path.Dir(relPath)
		if base != "." {
			if err := os.MkdirAll(path.Join(rootDir, base), 0700); err != nil && !os.IsExist(err) {
				return err
			}
		}
		f, err := os.Create(path.Join(rootDir, relPath))
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
