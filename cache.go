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
	if e.Files == nil {
		return []string{}
	}
	out := make([]string, 0, len(e.Files))
	for f, _ := range e.Files {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func CountSizeCache(i *EntryCache) int {
	countI := 1
	for _, v := range i.Files {
		countI += CountSizeCache(v)
	}
	return countI
}

func loadCache() (*os.File, *EntryCache, error) {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	cacheDir := path.Join(usr.HomeDir, ".dumbcas")
	if err := os.Mkdir(cacheDir, 0700); err != nil && !os.IsExist(err) {
		return nil, nil, fmt.Errorf("Failed to access %s: %s", cacheDir, err)
	}
	cache := &EntryCache{}
	cacheFile := path.Join(cacheDir, "cache.json")
	f, err := os.OpenFile(cacheFile, os.O_CREATE|os.O_RDWR, 0600)
	if f == nil {
		return nil, nil, fmt.Errorf("Failed to access %s: %s", cacheFile, err)
	}
	if data, err := ioutil.ReadAll(f); err == nil && len(data) != 0 {
		if err = json.Unmarshal(data, &cache); err != nil {
			// Ignore unmarshaling failure.
			cache = &EntryCache{}
		}
	}
	if _, err = f.Seek(0, 0); err != nil {
		return nil, nil, fmt.Errorf("Failed to seek %s: %s", cacheFile, err)
	}
	log.Printf("Loaded %d entries from the cache.", CountSizeCache(cache)-1)
	return f, cache, nil
}

func saveCache(f *os.File, cache *EntryCache) error {
	// TODO(maruel): When testing, the entries shouldn't be saved in the cache.
	// Trim anything > ~1yr old.
	one_year := time.Now().Unix() - (365 * 24 * 60 * 60)
	for relFile, file := range cache.Files {
		if file.LastTested < one_year {
			delete(cache.Files, relFile)
		}
	}
	log.Printf("Saving Cache: %d entries.", CountSizeCache(cache)-1)
	data, err := json.Marshal(&cache)
	if err != nil {
		return fmt.Errorf("Failed to marshall internal state: %s", err)
	}
	if err = f.Truncate(0); err != nil {
		return fmt.Errorf("Failed to truncate %s: %s", f.Name(), err)
	}
	if _, err = f.Write(data); err != nil {
		return fmt.Errorf("Failed to write %s: %s", f.Name(), err)
	}
	return nil
}
