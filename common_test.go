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
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"path"
)

// Logging is a global object so it can't be checked for when tests are run in parallel.
var bufLog bytes.Buffer

var enableOutput = false

func init() {
	// Reduces output. Comment out to get more logs.
	if !enableOutput {
		log.SetOutput(&bufLog)
	}
	log.SetFlags(log.Lmicroseconds)
}

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
func makeTempDir(name string) (string, error) {
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
		return tempFull, nil
	}
	return "", errors.New("Internal error")
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

func getLog(verbose bool) *log.Logger {
	var l *log.Logger
	if !enableOutput && !verbose {
		l = log.New(&bytes.Buffer{}, "", log.Lmicroseconds)
	} else {
		// Send directly to output for test debugging.
		l = log.New(os.Stderr, "", log.Lmicroseconds)
	}
	return l
}
