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
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
)

// Entry is an element. It can only contain the 2 firsts or the last one.
// TODO(maruel): Investigate if map[string]Entry could be used instead for
// performance reasons.
type Entry struct {
	Sha1  string            `json:"h,omitempty"`
	Size  int64             `json:"s,omitempty"`
	Files map[string]*Entry `json:"f,omitempty"`
}

// SortedFiles returns the child entry names sorted.
func (e *Entry) SortedFiles() []string {
	if e.Files == nil {
		return []string{}
	}
	out := make([]string, 0, len(e.Files))
	for f := range e.Files {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// CountMembers returns the number of all children elements recursively.
func (e *Entry) CountMembers() int {
	countI := 1
	for _, v := range e.Files {
		countI += v.CountMembers()
	}
	return countI
}

// Print prints the Entry in Yaml-inspired output.
func (e *Entry) Print(w io.Writer, indent string) {
	if e.Sha1 != "" {
		fmt.Fprintf(w, "%sSha1: %s\n", indent, e.Sha1)
		fmt.Fprintf(w, "%sSize: %d\n", indent, e.Size)
	}
	for _, f := range e.SortedFiles() {
		fmt.Fprintf(w, "%s- '%s'\n", indent, f)
		e.Files[f].Print(w, indent+"  ")
	}
}

func (e *Entry) isDir() bool {
	return e.Files != nil
}

type entryFileSystem struct {
	entry *Entry
	cas   CasTable
}

// "itemPath" must be posix-style.
func (e *entryFileSystem) pathToEntry(itemPath string) (*Entry, error) {
	if itemPath == "" || itemPath[0] != '/' {
		return nil, fmt.Errorf("internal error: %s is malformed", itemPath)
	}
	itemPath = strings.Trim(itemPath, "/")
	toServe := e.entry
	// Special case because strings.Split("", "/") returns []string{""}.
	if itemPath == "" {
		return toServe, nil
	}
	for _, item := range strings.Split(itemPath, "/") {
		if toServe.Files == nil {
			return nil, nil
		}
		if _, ok := toServe.Files[item]; !ok {
			return nil, nil
		}
		toServe = toServe.Files[item]
	}
	return toServe, nil
}

func (e *entryFileSystem) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "" || r.URL.Path[0] != '/' {
		http.Error(w, "Internal failure. entryFileSystem received an invalid url: "+r.URL.Path, http.StatusNotImplemented)
		return
	}

	// If so, it should be a dir.
	hasTrailing := strings.HasSuffix(r.URL.Path, "/")
	toServe, err := e.pathToEntry(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if toServe == nil {
		http.NotFound(w, r)
		return
	}

	if toServe.isDir() {
		if !hasTrailing {
			localRedirect(w, r, filepath.Base(r.URL.Path)+"/")
		} else {
			toServe.ServeDir(w)
		}
	} else {
		if hasTrailing {
			localRedirect(w, r, filepath.Base(r.URL.Path))
		} else {
			r.URL.Path = "/" + toServe.Sha1
			e.cas.ServeHTTP(w, r)
		}
	}
}

// ServeDir returns the child entries for an Entry.
func (e *Entry) ServeDir(w http.ResponseWriter) {
	names := make([]string, len(e.Files))
	i := 0
	for name, entry := range e.Files {
		if entry.isDir() {
			name = name + "/"
		}
		names[i] = name
		i++
	}
	dirList(w, names)
}
