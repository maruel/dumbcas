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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"
	"sort"
	"time"
)

type EntryCache struct {
	Sha1       string                 `json:"h,omitempty"`
	Size       int64                  `json:"s,omitempty"`
	Timestamp  int64                  `json:"t,omitempty"` // In Unix() epoch.
	LastTested int64                  `json:"T,omitempty"` // Last time this file was tested for presence.
	Files      map[string]*EntryCache `json:"f,omitempty"`
}

// Prints the EntryCache in Yaml-inspired output.
func (e *EntryCache) Print(w io.Writer, indent string) {
	if e.Sha1 != "" {
		fmt.Fprintf(w, "%sSha1: %s\n", indent, e.Sha1)
		fmt.Fprintf(w, "%sSize: %d\n", indent, e.Size)
	}
	for _, f := range e.SortedFiles() {
		fmt.Fprintf(w, "%s- '%s'\n", indent, f)
		e.Files[f].Print(w, indent+"  ")
	}
}

func (e *EntryCache) SortedFiles() []string {
	out := make([]string, 0, len(e.Files))
	for f, _ := range e.Files {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func (e *EntryCache) CountMembers() int {
	sum := 1
	for _, v := range e.Files {
		sum += v.CountMembers()
	}
	return sum
}

type cache struct {
	root *EntryCache
	f    *os.File
}

type Cache interface {
	Root() *EntryCache
	// Closes (and save) the cache.
	Close()
}

// Loads the cache from ~/.dumbcas/cache.json and keeps it open until the call
// to Save().
func LoadCache() (Cache, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, err
	}
	cacheDir := path.Join(usr.HomeDir, ".dumbcas")
	if err := os.Mkdir(cacheDir, 0700); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("Failed to access %s: %s", cacheDir, err)
	}
	root := &EntryCache{}
	cacheFile := path.Join(cacheDir, "cache.json")
	f, err := os.OpenFile(cacheFile, os.O_CREATE|os.O_RDWR, 0600)
	if f == nil {
		return nil, fmt.Errorf("Failed to access %s: %s", cacheFile, err)
	}
	if data, err := ioutil.ReadAll(f); err == nil && len(data) != 0 {
		if err = json.Unmarshal(data, &root); err != nil {
			// Ignore unmarshaling failure.
			root = &EntryCache{}
		}
	}
	if _, err = f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("Failed to seek %s: %s", cacheFile, err)
	}
	log.Printf("Loaded %d entries from the cache.", root.CountMembers()-1)
	return &cache{root, f}, nil
}

func (c *cache) Root() *EntryCache {
	return c.root
}

func (c *cache) Close() {
	// Trim anything > ~1yr old.
	defer func() {
		c.f.Close()
		c.f = nil
	}()

	one_year := time.Now().Unix() - (365 * 24 * 60 * 60)
	for relFile, file := range c.root.Files {
		if file.LastTested < one_year {
			delete(c.root.Files, relFile)
		}
	}
	log.Printf("Saving Cache: %d entries.", c.root.CountMembers()-1)
	data, err := json.Marshal(c.root)
	if err != nil {
		log.Printf("Failed to marshall internal state: %s", err)
		return
	}
	if err = c.f.Truncate(0); err != nil {
		log.Printf("Failed to truncate %s: %s", c.f.Name(), err)
		return
	}
	if _, err = c.f.Write(data); err != nil {
		log.Printf("Failed to write %s: %s", c.f.Name(), err)
		return
	}
}
