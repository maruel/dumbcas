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
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

func init() {
	gob.Register(&EntryCache{})
}

// Describe an entry in the cache. Can be either a file or a directory. Using
// this structure is more compact than a flat list for deep trees.
type EntryCache struct {
	Sha1       string
	Size       int64
	Timestamp  int64 // In Unix() epoch.
	LastTested int64 // Last time this file was tested for presence.
	Files      map[string]*EntryCache
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
	root     *EntryCache
	filePath string
	log      *log.Logger
}

type Cache interface {
	// Returns the root entry. Must be non-nil.
	Root() *EntryCache
	// Closes (and save) the cache.
	Close()
}

// Finds an item in the cache or create it if not present.
func FindInCache(c Cache, itemPath string) *EntryCache {
	if filepath.Separator == '/' && itemPath[0] == '/' {
		itemPath = itemPath[1:]
	}
	entry := c.Root()
	for _, p := range strings.Split(itemPath, string(filepath.Separator)) {
		if entry.Files == nil {
			entry.Files = make(map[string]*EntryCache)
		}
		if entry.Files[p] == nil {
			entry.Files[p] = &EntryCache{}
		}
		entry = entry.Files[p]
	}
	return entry
}

func getCachePath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return path.Join(usr.HomeDir, ".dumbcas"), nil
}

// Loads the cache from ~/.dumbcas/cache.json and keeps it open until the call
// to Save(). It is guaranteed to return a non-nil Cache instance even in case
// of failure to load the cache from disk and that error is non-nil.
//
// TODO(maruel): Ensure proper file locking. One way is to always create a new
// file when adding data and then periodically garbage-collect the files.
func loadCache(l *log.Logger) (Cache, error) {
	cacheDir, err := getCachePath()
	if err != nil {
		return &cache{&EntryCache{}, "", l}, err
	}
	return loadCacheInner(cacheDir, l)
}

func loadCacheInner(cacheDir string, l *log.Logger) (Cache, error) {
	cache := &cache{&EntryCache{}, path.Join(cacheDir, "cache.gob"), l}
	if err := os.Mkdir(cacheDir, 0700); err != nil && !os.IsExist(err) {
		return cache, fmt.Errorf("Failed to access %s: %s", cacheDir, err)
	}
	f, err := os.OpenFile(cache.filePath, os.O_RDONLY, 0600)
	if f == nil {
		if os.IsNotExist(err) {
			// Do not this as an error, it would be confusing.
			return cache, nil
		}
		return cache, fmt.Errorf("Failed to access %s: %s", cache.filePath, err)
	}
	defer f.Close()
	// The cache uses gob instead of json because:
	// - The data can be read and written incrementally instead of having to read
	//   it all at once.
	// - It's significantly faster than json.
	// - It's significantly smaller than json.
	// - The program works fine without cache so it's not a big deal if it ever
	//   become backward incomatible.
	d := gob.NewDecoder(f)
	if err := d.Decode(cache.root); err != nil && err != io.EOF {
		// Ignore unmarshaling failure by reseting the content. Better be safe than
		// sorry.
		cache.log.Printf("Failed loading cache: %s", err)
		cache.root = &EntryCache{}
	}
	cache.log.Printf("Loaded %d entries from the cache.", cache.root.CountMembers()-1)
	return cache, nil
}

func (c *cache) Root() *EntryCache {
	return c.root
}

func (c *cache) Close() {
	if c.filePath == "" {
		return
	}
	f, err := os.OpenFile(c.filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if f == nil {
		c.log.Printf("Failed to save cache %s: %s", c.filePath, err)
		return
	}
	// TODO(maruel): Trim anything > ~1yr old.
	c.log.Printf("Saving Cache: %d entries.", c.root.CountMembers()-1)
	e := gob.NewEncoder(f)
	if err := e.Encode(c.root); err != nil {
		c.log.Printf("Failed to write %s: %s", c.filePath, err)
	}
	f.Close()
	stat, err := os.Stat(c.filePath)
	if err != nil {
		c.log.Printf("Unexpected error while stat'ing %s: %s", c.filePath, err)
	} else if stat.Size() < 100 {
		c.log.Printf("Failed to serialize %s: %d", c.filePath, stat.Size())
	}
}
