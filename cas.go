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
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
)

const casName = "cas"
const needFsckName = "need_fsck"

// Creates 16^3 (4096) directories. Preferable values are 2 or 3.
const splitAt = 3

type CasTable struct {
	rootDir      string
	casDir       string
	prefixLength int
	validPath    *regexp.Regexp
}

func (c *CasTable) RelPath(hash string) string {
	return path.Join(casName, hash[:c.prefixLength], hash[c.prefixLength:])
}

// Converts an entry in the table into a proper file path.
func (c *CasTable) FilePath(hash string) string {
	match := c.validPath.FindStringSubmatch(hash)
	if match == nil {
		log.Printf("filePath(%s) is invalid", hash)
		return ""
	}
	fullPath := path.Join(c.casDir, match[0][:c.prefixLength], match[0][c.prefixLength:])
	if !path.IsAbs(fullPath) {
		log.Printf("filePath(%s) is invalid", hash)
		return ""
	}
	return fullPath
}

func PrefixSpace(prefixLength uint) int {
	if prefixLength == 0 {
		return 0
	}
	return 1<<(prefixLength*4)
}

func MakeCasTable(rootDir string) (*CasTable, error) {
	//log.Printf("MakeCasTable(%s)", rootDir)
	if !path.IsAbs(rootDir) {
		return nil, fmt.Errorf("MakeCasTable(%s) is not valid", rootDir)
	}
	rootDir = path.Clean(rootDir)
	casDir := path.Join(rootDir, casName)
	prefixLength := splitAt
	if err := os.Mkdir(casDir, 0750); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("MakeCasTable(%s): failed to create %s: %s", casDir, err)
	} else if !os.IsExist(err) {
		// Create all the prefixes at initialization time so they don't need to be
		// tested all the time.
		for i := 0; i < PrefixSpace(uint(prefixLength)); i++ {
			prefix := fmt.Sprintf("%0*x", prefixLength, i)
			if err := os.Mkdir(path.Join(casDir, prefix), 0750); err != nil && !os.IsExist(err) {
				return nil, fmt.Errorf("Failed to create %s: %s\n", prefix, err)
			}
		}
	}
	return &CasTable{
		rootDir,
		casDir,
		prefixLength,
		regexp.MustCompile("^([a-f0-9]{40})$"),
	}, nil
}

// Expects the format "/<hash>". In particular, refuses "/<hash>/".
func (c *CasTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//log.Printf("CasTable.ServeHTTP(%s)", r.URL.Path)
	if r.URL.Path == "" || r.URL.Path[0] != '/' {
		http.Error(w, "Internal failure. CasTable received an invalid url: "+r.URL.Path, http.StatusNotImplemented)
		return
	}
	casItem := c.FilePath(r.URL.Path[1:])
	if casItem == "" {
		http.Error(w, "Invalid CAS url: "+r.URL.Path, http.StatusBadRequest)
		return
	}
	http.ServeFile(w, r, casItem)
}

type Item struct {
	Item    string
	Invalid string
	Error   error
}

// Enumerates all the entries in the CAS.
func (c *CasTable) Enumerate(items chan<- Item) {
	rePrefix := regexp.MustCompile(fmt.Sprintf("^[a-f0-9]{%d}$", c.prefixLength))
	reRest := regexp.MustCompile(fmt.Sprintf("^[a-f0-9]{%d}$", 40-c.prefixLength))

	// TODO(maruel): No need to read all at once.
	prefixes, err := readDirNames(c.casDir)
	if err != nil {
		items <- Item{Error: fmt.Errorf("Failed reading ss", c.casDir)}
		return
	}

	for _, prefix := range prefixes {
		if !rePrefix.MatchString(prefix) {
			items <- Item{Invalid: path.Join(casName, prefix)}
			c.NeedFsck()
			continue
		}
		// TODO(maruel): No need to read all at once.
		prefixPath := path.Join(c.casDir, prefix)
		subitems, err := readDirNames(prefixPath)
		if err != nil {
			items <- Item{Error: fmt.Errorf("Failed reading %s", prefixPath)}
			c.NeedFsck()
			continue
		}
		for _, item := range subitems {
			if !reRest.MatchString(item) {
				items <- Item{Invalid: path.Join(casName, prefix, item)}
				c.NeedFsck()
				continue
			}
			items <- Item{Item: prefix + item}
		}
	}
	items <- Item{}
}

// Signals that an fsck is required.
func (c *CasTable) NeedFsck() {
	log.Printf("Marking for fsck")
	f, _ := os.Create(path.Join(c.casDir, needFsckName))
	if f != nil {
		f.Close()
	}
}

func (c *CasTable) WarnIfFsckIsNeeded() bool {
	f, _ := os.Open(path.Join(c.casDir, needFsckName))
	if f == nil {
		return false
	}
	defer f.Close()
	fmt.Fprintf(os.Stderr, "WARNING: fsck is needed.")
	return true
}
