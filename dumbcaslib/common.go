/* Copyright 2012 Marc-Antoine Ruel. Licensed under the Apache License, Version
2.0 (the "License"); you may not use this file except in compliance with the
License.  You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0. Unless required by applicable law or
agreed to in writing, software distributed under the License is distributed on
an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
or implied. See the License for the specific language governing permissions and
limitations under the License. */

package dumbcaslib

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/maruel/interrupt"
)

// Table represents a flat table of data.
type Table interface {
	// Must be able to efficiently respond to an HTTP GET request.
	http.Handler
	// Enumerate enumerates all the entries in the table.
	Enumerate() <-chan EnumerationEntry
	// Open opens an entry for reading.
	Open(name string) (ReadSeekCloser, error)
	// Remove removes a node enumerated by Enumerate().
	Remove(name string) error
}

// Corruptable is implemented only by in-memory implementations for unit
// testing fsck-like algorithms.
type Corruptable interface {
	// Corrupt corrupts an element in the table.
	Corrupt()
}

// EnumerationEntry is one element in the enumeration functions.
type EnumerationEntry struct {
	Item  string
	Error error
}

// ReadSeekCloser implements all of io.Reader, io.Seeker and io.Closer.
type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

// TreeItem is an item returned by EnumerateTree.
type TreeItem struct {
	FullPath string
	os.FileInfo
	Error error
}

func recurseEnumerateTree(rootDir string, c chan<- TreeItem) bool {
	f, err := os.Open(rootDir)
	if err != nil {
		c <- TreeItem{Error: err}
		return false
	}
	defer func() {
		_ = f.Close()
	}()
	for {
		if interrupt.IsSet() {
			break
		}
		dirs, err := f.Readdir(128)
		if err != nil && err != io.EOF {
			c <- TreeItem{Error: err}
			return false
		}
		if len(dirs) == 0 {
			break
		}
		for _, d := range dirs {
			if interrupt.IsSet() {
				break
			}
			name := d.Name()
			fullPath := filepath.Join(rootDir, name)
			if d.IsDir() {
				if !recurseEnumerateTree(fullPath, c) {
					return false
				}
			} else {
				c <- TreeItem{FullPath: fullPath, FileInfo: d}
			}
		}
	}
	return true
}

// EnumerateTree walks the directory tree.
func EnumerateTree(rootDir string) <-chan TreeItem {
	c := make(chan TreeItem)
	go func() {
		recurseEnumerateTree(rootDir, c)
		close(c)
	}()
	return c
}

func isDir(path string) bool {
	stat, _ := os.Stat(path)
	return stat != nil && stat.IsDir()
}

// Reads a directory list and guarantees to return a list.
func readDirNames(dirPath string) ([]string, error) {
	f, err := os.Open(dirPath)
	if err != nil {
		return []string{}, err
	}
	defer func() {
		_ = f.Close()
	}()
	return f.Readdirnames(0)
}

// Reads a directory list and guarantees to return a list.
func readDirFancy(dirPath string) ([]string, error) {
	names := []string{}
	f, err := os.Open(dirPath)
	if err != nil {
		return names, err
	}
	defer func() {
		_ = f.Close()
	}()
	for {
		dirs, err := f.Readdir(1024)
		if err != nil || len(dirs) == 0 {
			break
		}
		for _, d := range dirs {
			name := d.Name()
			if d.IsDir() {
				name += "/"
			}
			names = append(names, name)
		}
	}
	return names, err
}

// Sha1Bytes returns the hex encoded SHA-1 from the content.
func Sha1Bytes(content []byte) string {
	hash := sha1.New()
	_, _ = hash.Write(content)
	return hex.EncodeToString(hash.Sum(nil))
}

// LoadReaderAsJSON decodes JSON data from a io.Reader.
func LoadReaderAsJSON(r io.Reader, value interface{}) error {
	data, err := ioutil.ReadAll(r)
	if err == nil {
		return json.Unmarshal(data, &value)
	}
	return err
}

func loadFileAsJSON(filepath string, value interface{}) error {
	f, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("loadFileAsJSON(%s): %s", filepath, err)
	}
	defer func() {
		_ = f.Close()
	}()
	return LoadReaderAsJSON(f, value)
}

type closableBuffer struct {
	*bytes.Reader
}

func (b closableBuffer) Close() error {
	return nil
}
