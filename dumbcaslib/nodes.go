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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Node is a element in the index NodesTable.
type Node struct {
	Entry   string
	Comment string `json:",omitempty"`
}

// NodesTable is an index to a CasTable.
type NodesTable interface {
	Table
	// AddEntry adds a node to the table.
	AddEntry(node *Node, name string) (string, error)
}

// EnumerateNodesAsList returns a sorted list of all the entries. It is means
// for testing.
func EnumerateNodesAsList(nodes NodesTable) ([]string, error) {
	items := []string{}
	for v := range nodes.Enumerate() {
		if v.Error != nil {
			return nil, v.Error
		}
		items = append(items, v.Item)
	}
	sort.Strings(items)
	return items, nil
}

type memoryNodesTable struct {
	lock    sync.Mutex
	entries map[string][]byte
	cas     CasTable
}

// MakeMemoryNodesTable returns a NodeTable implementation all in memory.
func MakeMemoryNodesTable(cas CasTable) NodesTable {
	return &memoryNodesTable{entries: make(map[string][]byte), cas: cas}
}

func (m *memoryNodesTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	suburl := r.URL.Path[1:]
	if suburl != "" {
		// Slow search, it's fine for a fake.
		for k, v := range m.entries {
			k = strings.Replace(k, string(filepath.Separator), "/", -1)
			if strings.HasPrefix(suburl, k) {
				// Found.
				rest := suburl[len(k):]
				if rest == "" {
					// TODO(maruel): posix-specific.
					localRedirect(w, r, path.Base(r.URL.Path)+"/")
					return
				}

				node := &Node{}
				if err := json.Unmarshal(v, node); err != nil {
					http.Error(w, fmt.Sprintf("Failed to load the entry file: %s", err), http.StatusNotFound)
					return
				}
				entry, err := LoadEntry(m.cas, node.Entry)
				if err != nil {
					http.Error(w, fmt.Sprintf("Failed to load the entry file: %s", err), http.StatusNotFound)
					return
				}
				// Defer to the cas file system.
				r.URL.Path = rest
				entryFs := entryFileSystem{cas: m.cas, entry: entry}
				entryFs.ServeHTTP(w, r)
				return
			}
		}
	}

	needRedirect := !strings.HasSuffix(r.URL.Path, "/")
	if needRedirect {
		suburl += "/"
	}

	// List the corresponding "directory", if found.
	items := []string{}
	for k := range m.entries {
		k = strings.Replace(k, string(filepath.Separator), "/", -1)
		if strings.HasPrefix(k, suburl) {
			v := strings.SplitAfterN(k[len(suburl):], "/", 2)
			items = append(items, v[0])
		}
	}
	if len(items) != 0 {
		if needRedirect {
			// Not strictly valid but fine enough for a fake.
			localRedirect(w, r, path.Base(r.URL.Path)+"/")
			return
		}
		dirList(w, items)
		return
	}
	http.Error(w, "Yo dawg", http.StatusNotFound)
}

func (m *memoryNodesTable) AddEntry(node *Node, name string) (string, error) {
	data, err := json.Marshal(node)
	if err != nil {
		return "", fmt.Errorf("Failed to marshall internal state: %s", err)
	}

	now := time.Now().UTC()
	monthName := now.Format("2006-01")

	nodePath := ""
	suffix := 0
	for {
		nodeName := now.Format("2006-01-02_15-04-05") + "_" + name
		if suffix != 0 {
			nodeName += fmt.Sprintf("(%d)", suffix)
		}
		nodePath = filepath.Join(monthName, nodeName)
		if _, ok := m.entries[nodePath]; !ok {
			m.entries[nodePath] = data
			break
		}
		// Try ad nauseam.
		suffix++
	}
	// The real implementation creates a symlink if possible.
	m.entries[tagsName+"/"+name] = data
	return nodePath, nil
}

func (m *memoryNodesTable) Enumerate() <-chan EnumerationEntry {
	c := make(chan EnumerationEntry)
	go func() {
		m.lock.Lock()
		entries := make(map[string][]byte)
		for k, v := range m.entries {
			entries[k] = v
		}
		m.lock.Unlock()
		for k := range entries {
			c <- EnumerationEntry{Item: k}
		}
		close(c)
	}()
	return c
}

func (m *memoryNodesTable) Open(item string) (ReadSeekCloser, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	data, ok := m.entries[item]
	if !ok {
		return nil, fmt.Errorf("Missing: %s", item)
	}
	return closableBuffer{bytes.NewReader(data)}, nil
}

func (m *memoryNodesTable) Remove(name string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.entries[name]; !ok {
		return os.ErrNotExist
	}
	delete(m.entries, name)
	return nil
}

func (m *memoryNodesTable) Corrupt() {
	m.entries["tags/fictious"] = []byte("Invalid JSON")
}
