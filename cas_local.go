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
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

const casName = "cas"
const needFsckName = "need_fsck"

type casTable struct {
	rootDir      string
	casDir       string
	prefixLength int
	hashLength   int
	validPath    *regexp.Regexp
	trash        Trash
}

// Converts an entry in the table into a proper file path.
func (c *casTable) filePath(hash string) string {
	match := c.validPath.FindStringSubmatch(hash)
	if match == nil {
		log.Printf("filePath(%s) is invalid", hash)
		return ""
	}
	fullPath := filepath.Join(c.casDir, match[0][:c.prefixLength], match[0][c.prefixLength:])
	if !filepath.IsAbs(fullPath) {
		log.Printf("filePath(%s) is invalid", hash)
		return ""
	}
	return fullPath
}

func prefixSpace(prefixLength uint) int {
	if prefixLength == 0 {
		return 0
	}
	return 1 << (prefixLength * 4)
}

func makeLocalCasTable(rootDir string) (CasTable, error) {
	//log.Printf("makeCasTable(%s)", rootDir)
	// Creates 16^3 (4096) directories. Preferable values are 2 or 3.
	prefixLength := 3
	// Currently hardcoded for SHA-1 but could be used for any length.
	hashLength := sha1.Size * 2

	if !filepath.IsAbs(rootDir) {
		return nil, fmt.Errorf("MakeCasTable(%s) is not valid", rootDir)
	}
	rootDir = filepath.Clean(rootDir)
	casDir := filepath.Join(rootDir, casName)
	if err := os.MkdirAll(casDir, 0750); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("MakeCasTable(%s): failed to create the directory: %s", casDir, err)
	} else if !os.IsExist(err) {
		// Create all the prefixes at initialization time so they don't need to be
		// tested all the time.
		for i := 0; i < prefixSpace(uint(prefixLength)); i++ {
			prefix := fmt.Sprintf("%0*x", prefixLength, i)
			if err := os.Mkdir(filepath.Join(casDir, prefix), 0750); err != nil && !os.IsExist(err) {
				return nil, fmt.Errorf("Failed to create %s: %s\n", prefix, err)
			}
		}
	}
	return &casTable{
		rootDir,
		casDir,
		prefixLength,
		hashLength,
		regexp.MustCompile(fmt.Sprintf("^([a-f0-9]{%d})$", hashLength)),
		MakeTrash(casDir),
	}, nil
}

// Expects the format "/<hash>". In particular, refuses "/<hash>/".
func (c *casTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//log.Printf("casTable.ServeHTTP(%s)", r.URL.Path)
	if r.URL.Path == "" || r.URL.Path[0] != '/' {
		http.Error(w, "Internal failure. CasTable received an invalid url: "+r.URL.Path, http.StatusNotImplemented)
		return
	}
	casItem := c.filePath(r.URL.Path[1:])
	if casItem == "" {
		http.Error(w, "Invalid CAS url: "+r.URL.Path, http.StatusBadRequest)
		return
	}
	http.ServeFile(w, r, casItem)
}

// Enumerates all the entries in the table. If a file or directory is found in
// the directory tree that doesn't match the expected format, it will be moved
// into the trash.
func (c *casTable) Enumerate() <-chan EnumerationEntry {
	rePrefix := regexp.MustCompile(fmt.Sprintf("^[a-f0-9]{%d}$", c.prefixLength))
	reRest := regexp.MustCompile(fmt.Sprintf("^[a-f0-9]{%d}$", c.hashLength-c.prefixLength))
	items := make(chan EnumerationEntry)

	// TODO(maruel): No need to read all at once.
	go func() {
		prefixes, err := readDirNames(c.casDir)
		if err != nil {
			items <- EnumerationEntry{Error: fmt.Errorf("Failed reading ss", c.casDir)}
		} else {
			for _, prefix := range prefixes {
				if IsInterrupted() {
					break
				}
				if prefix == TrashName {
					continue
				}
				if !rePrefix.MatchString(prefix) {
					_ = c.trash.Move(prefix)
					c.SetFsckBit()
					continue
				}
				// TODO(maruel): No need to read all at once.
				prefixPath := filepath.Join(c.casDir, prefix)
				subitems, err := readDirNames(prefixPath)
				if err != nil {
					items <- EnumerationEntry{Error: fmt.Errorf("Failed reading %s", prefixPath)}
					c.SetFsckBit()
					continue
				}
				for _, item := range subitems {
					if !reRest.MatchString(item) {
						_ = c.trash.Move(filepath.Join(prefix, item))
						c.SetFsckBit()
						continue
					}
					items <- EnumerationEntry{Item: prefix + item}
				}
			}
		}
		close(items)
	}()
	return items
}

// Adds an entry with the hash calculated already if not alreaady present. It's
// a performance optimization to be able to not write the object unless needed.
func (c *casTable) AddEntry(source io.Reader, hash string) error {
	dst := c.filePath(hash)
	df, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
	if os.IsExist(err) {
		return err
	}
	if err != nil {
		return fmt.Errorf("Failed to copy(dst) %s: %s", dst, err)
	}
	defer func() {
		_ = df.Close()
	}()
	_, err = io.Copy(df, source)
	return err
}

func (c *casTable) Open(hash string) (ReadSeekCloser, error) {
	fp := c.filePath(hash)
	if fp == "" {
		return nil, os.ErrInvalid
	}
	return os.Open(fp)
}

func (c *casTable) SetFsckBit() {
	log.Printf("Marking for fsck")
	f, _ := os.Create(filepath.Join(c.casDir, needFsckName))
	if f != nil {
		_ = f.Close()
	}
}

func (c *casTable) GetFsckBit() bool {
	f, _ := os.Open(filepath.Join(c.casDir, needFsckName))
	if f == nil {
		return false
	}
	_ = f.Close()
	return true
}

func (c *casTable) ClearFsckBit() {
	_ = os.Remove(filepath.Join(c.casDir, needFsckName))
}

func (c *casTable) Remove(hash string) error {
	match := c.validPath.FindStringSubmatch(hash)
	if match == nil {
		return fmt.Errorf("Remove(%s) is invalid", hash)
	}
	return c.trash.Move(filepath.Join(hash[:c.prefixLength], hash[c.prefixLength:]))
}

// Utility function when the data is already in memory but not yet hashed.
func AddBytes(c CasTable, data []byte) (string, error) {
	hash := sha1Bytes(data)
	return hash, c.AddEntry(bytes.NewBuffer(data), hash)
}
