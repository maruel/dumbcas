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
	"html"
	"io"
	"log"
	"net/http"
	"path"
	"sort"
	"strings"
)

// Can only contain the 2 firsts or the last one.
// TODO(maruel): Investigate if map[string]Entry could be used instead for
// performance reasons.
type Entry struct {
	Sha1  string            `json:"h,omitempty"`
	Size  int64             `json:"s,omitempty"`
	Files map[string]*Entry `json:"f,omitempty"`
}

func (e *Entry) SortedFiles() []string {
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

// Prints the Entry in Yaml-inspired output.
func (e *Entry) Print(w io.Writer, indent string) {
	if e.Sha1 != "" {
		fmt.Fprintf(w, "%sSha1: %s\n", indent, e.Sha1)
		fmt.Fprintf(w, "%sSize: %d\n", indent, e.Size)
	}
	if e.Files != nil {
		for _, f := range e.SortedFiles() {
			fmt.Fprintf(w, "%s- '%s'\n", indent, f)
			e.Files[f].Print(w, indent+"  ")
		}
	}
}

func (e *Entry) isDir() bool {
	return e.Files != nil
}

type EntryFileSystem struct {
	entry *Entry
	cas   *CasTable
}

// "itemPath" must be posix-style.
func (e *EntryFileSystem) pathToEntry(itemPath string) *Entry {
	if itemPath == "" || itemPath[0] != '/' {
		log.Printf("Internal error.")
		return nil
	}
	itemPath = strings.Trim(itemPath, "/")
	toServe := e.entry
	// Special case because strings.Split("", "/") returns []string{""}.
	if itemPath == "" {
		return toServe
	}
	for _, item := range strings.Split(itemPath, "/") {
		if toServe.Files == nil {
			return nil
		}
		if _, ok := toServe.Files[item]; !ok {
			return nil
		}
		toServe = toServe.Files[item]
	}
	return toServe
}

func (e *EntryFileSystem) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//log.Printf("EntryFileSystem.ServeHTTP(%s)", r.URL.Path)
	if r.URL.Path == "" || r.URL.Path[0] != '/' {
		http.Error(w, "Internal failure. EntryFileSystem received an invalid url: "+r.URL.Path, http.StatusNotImplemented)
		return
	}

	// If so, it should be a dir.
	hasTrailing := strings.HasSuffix(r.URL.Path, "/")
	toServe := e.pathToEntry(r.URL.Path)
	if toServe == nil {
		http.NotFound(w, r)
		return
	}

	if toServe.isDir() {
		if !hasTrailing {
			localRedirect(w, r, path.Base(r.URL.Path)+"/")
		} else {
			toServe.ServeDir(w)
		}
	} else {
		if hasTrailing {
			localRedirect(w, r, path.Base(r.URL.Path))
		} else {
			r.URL.Path = "/" + toServe.Sha1
			e.cas.ServeHTTP(w, r)
		}
	}
}

func (e *Entry) ServeDir(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "<html><body><pre>")
	names := make([]string, 0, len(e.Files))
	for name, entry := range e.Files {
		if entry.isDir() {
			name = name + "/"
		}
		name = html.EscapeString(name)
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(w, "<a href=\"%s\">%s</a>\n", name, name)
	}
	io.WriteString(w, "</pre></body></html>")
}
