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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

type Table interface {
	// Must be able to efficiently respond to an HTTP GET request.
	http.Handler
	// Enumerates all the entries in the table.
	Enumerate() <-chan EnumerationEntry
	// Opens an entry for reading.
	Open(name string) (ReadSeekCloser, error)
	// Removes a node enumerated by Enumerate().
	Remove(name string) error
}

type EnumerationEntry struct {
	Item  string
	Error error
}

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

// Common flags.
type CommonFlags struct {
	CommandRunBase
	Root  string
	cas   CasTable
	nodes NodesTable
}

func (c *CommonFlags) Init() {
	c.Flags.StringVar(&c.Root, "root", os.Getenv("DUMBCAS_ROOT"), "Root directory; required. Set $DUMBCAS_ROOT to set a default.")
}

func (c *CommonFlags) Parse(d DumbcasApplication, bypassFsck bool) error {
	if c.Root == "" {
		return errors.New("Must provide -root")
	}
	if root, err := filepath.Abs(c.Root); err != nil {
		return fmt.Errorf("Failed to find %s", c.Root)
	} else {
		c.Root = root
	}

	if cas, err := d.MakeCasTable(c.Root); err != nil {
		return err
	} else {
		c.cas = cas
	}

	if c.cas.GetFsckBit() {
		if !bypassFsck {
			return fmt.Errorf("Can't run if fsck is needed. Please run fsck first.")
		}
		fmt.Fprintf(os.Stderr, "WARNING: fsck is needed.")
	}
	if nodes, err := d.LoadNodesTable(c.Root, c.cas); err != nil {
		return err
	} else {
		c.nodes = nodes
	}
	return nil
}

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
	defer f.Close()
	for {
		if IsInterrupted() {
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
			if IsInterrupted() {
				break
			}
			name := d.Name()
			fullPath := path.Join(rootDir, name)
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

// Walk the directory tree.
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
	defer f.Close()
	return f.Readdirnames(0)
}

// Reads a directory list and guarantees to return a list.
func readDirFancy(dirPath string) ([]string, error) {
	names := []string{}
	f, err := os.Open(dirPath)
	if err != nil {
		return names, err
	}
	defer f.Close()
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

func sha1File(f io.Reader) (string, error) {
	hash := sha1.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func sha1FilePath(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return sha1File(f)
}

func sha1Bytes(content []byte) string {
	hash := sha1.New()
	hash.Write(content)
	return hex.EncodeToString(hash.Sum(nil))
}

func loadReaderAsJson(r io.Reader, value interface{}) error {
	data, err := ioutil.ReadAll(r)
	if err == nil {
		return json.Unmarshal(data, &value)
	}
	return err
}

func loadFileAsJson(filepath string, value interface{}) error {
	f, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("loadFileAsJson(%s): %s", filepath, err)
	}
	defer f.Close()
	return loadReaderAsJson(f, value)
}
