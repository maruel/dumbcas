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
	"testing"
)

type mockNodesTable struct {
	entries map[string]*Node
	cas     CasTable
	t       *testing.T
	log     *log.Logger
}

func (a *ApplicationMock) LoadNodesTable(rootDir string, cas CasTable) (NodesTable, error) {
	return loadNodesTable(rootDir, cas, a.GetLog())
	if a.nodes == nil {
		a.nodes = &mockNodesTable{make(map[string]*Node), a.cas, a.T, a.log}
	}
	return a.nodes, nil
}

func (m *mockNodesTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.log.Printf("mockNodesTable.ServeHTTP(%s)", r.URL.Path)
	fmt.Fprintf(w, "TODO")
	//w.Write(m.entries[r.URL.Path[1:]])
}

func (m *mockNodesTable) AddEntry(node *Node, name string) error {
	m.log.Printf("mockNodesTable.AddEntry(%s)", name)
	m.entries[name] = nil
	return nil
}

func (m *mockNodesTable) Enumerate() <-chan NodeEntry {
	m.log.Printf("mockNodesTable.Enumerate() %d", len(m.entries))
	c := make(chan NodeEntry)
	go func() {
		// TODO(maruel): Will blow up if mutated concurrently.
		for k, v := range m.entries {
			c <- NodeEntry{Path: k, Node: v, Entry: nil}
		}
		close(c)
	}()
	return c
}
