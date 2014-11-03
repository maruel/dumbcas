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
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

func init() {
	gob.Register(&EntryCache{})
}

// EntryCache describes an entry in the cache. Can be either a file or a
// directory. Using this structure is more compact than a flat list for deep
// trees.
type EntryCache struct {
	Sha1       string
	Size       int64
	Timestamp  int64 // In Unix() epoch.
	LastTested int64 // Last time this file was tested for presence.
	Files      map[string]*EntryCache
}

// Print prints the EntryCache in Yaml-inspired output.
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

// SortedFiles returns the entries in this directory sorted.
func (e *EntryCache) SortedFiles() []string {
	out := make([]string, 0, len(e.Files))
	for f := range e.Files {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// CountMembers returns the number of all items in this tree recursively.
func (e *EntryCache) CountMembers() int {
	sum := 1
	for _, v := range e.Files {
		sum += v.CountMembers()
	}
	return sum
}

// Cache is a cache to entries to speed up adding elements to a CasTable.
type Cache interface {
	io.Closer

	// Returns the root entry. Must be non-nil.
	Root() *EntryCache
}

// FindInCache finds an item in the cache or create it if not present.
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

type memoryCache struct {
	root   *EntryCache
	closed bool
}

// MakeMemoryCache returns an in-memory Cache implementation. Useful for
// testing.
func MakeMemoryCache() Cache {
	return &memoryCache{&EntryCache{}, false}
}

func (m *memoryCache) Root() *EntryCache {
	if m.closed {
		panic("was unexpectedly closed")
	}
	return m.root
}

func (m *memoryCache) Close() error {
	if m.closed {
		return errors.New("was unexpectedly closed twice")
	}
	m.closed = false
	return nil
}
