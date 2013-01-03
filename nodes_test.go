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
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type mockNodesTable struct {
	entries map[string]Node
	cas     CasTable
	t       *testing.T
	log     *log.Logger
}

func (a *ApplicationMock) LoadNodesTable(rootDir string, cas CasTable) (NodesTable, error) {
	return loadNodesTable(rootDir, cas, a.GetLog())
	if a.nodes == nil {
		a.nodes = &mockNodesTable{make(map[string]Node), a.cas, a.T, a.log}
	}
	return a.nodes, nil
}

func (m *mockNodesTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.log.Printf("mockNodesTable.ServeHTTP(%s)", r.URL.Path)
	suburl := r.URL.Path[1:]
	if suburl == "" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Yo dawg")
		return
	}
	// Slow search, it's fine for a mock.
	for k, v := range m.entries {
		if strings.HasPrefix(k, suburl) {
			// Found.
			f, err := m.cas.Open(v.Entry)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to load the entry file: %s", err), http.StatusNotFound)
				return
			}
			defer f.Close()
			entryFs := EntryFileSystem{cas: m.cas}
			if err := loadReaderAsJson(f, &entryFs.entry); err != nil {
				http.Error(w, fmt.Sprintf("Failed to load the entry file: %s", err), http.StatusNotFound)
				return
			}
			// Defer to the cas file system.
			r.URL.Path = suburl[len(k)+1:]
			entryFs.ServeHTTP(w, r)
		}
	}
	http.Error(w, "Yo dawg", http.StatusNotFound)
}

func (m *mockNodesTable) AddEntry(node *Node, name string) error {
	m.log.Printf("mockNodesTable.AddEntry(%s)", name)

	now := time.Now().UTC()
	monthName := now.Format("2006-01")

	suffix := 0
	for {
		nodeName := now.Format("2006-01-02_15-04-05") + "_" + name
		if suffix != 0 {
			nodeName += fmt.Sprintf("(%d)", suffix)
		}
		nodePath := monthName + "/" + nodeName
		if _, ok := m.entries[nodePath]; !ok {
			m.entries[nodePath] = *node
			break
		}
		// Try ad nauseam.
		suffix += 1
	}
	m.entries[tagsName+"/"+name] = *node
	return nil
}

func (m *mockNodesTable) Enumerate() <-chan NodeEntry {
	m.log.Printf("mockNodesTable.Enumerate() %d", len(m.entries))
	c := make(chan NodeEntry)
	go func() {
		// TODO(maruel): Will blow up if mutated concurrently.
		for k, v := range m.entries {
			c <- NodeEntry{Path: k, Node: &v, Entry: nil}
		}
		close(c)
	}()
	return c
}

func TestNodesTable(t *testing.T) {
	t.Parallel()
	tempData, err := makeTempDir("nodes")
	if err != nil {
		t.Fatalf("Failed to create tempdir", err)
	}
	defer removeTempDir(tempData)

	log := getLog(false)
	cas := &mockCasTable{make(map[string][]byte), false, t, log}
	load := func() (NodesTable, error) {
		return loadNodesTable(tempData, cas, log)
	}
	testNodesTableImpl(t, load)
}

func TestNodesTableMock(t *testing.T) {
	t.Parallel()
	log := getLog(false)
	cas := &mockCasTable{make(map[string][]byte), false, t, log}
	nodes := &mockNodesTable{make(map[string]Node), cas, t, log}
	load := func() (NodesTable, error) {
		return nodes, nil
	}
	testNodesTableImpl(t, load)
}

func testNodesTableImpl(t *testing.T, load func() (NodesTable, error)) {
	nodes, err := load()
	if err != nil {
		t.Fatal(err)
	}

	for _ = range nodes.Enumerate() {
		t.Fatal("Found unexpected value")
	}
	if err := nodes.AddEntry(&Node{"entry sha1", "comment"}, "name"); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _ = range nodes.Enumerate() {
		count += 1
	}
	if count != 2 {
		t.Fatalf("Found %d items", count)
	}

	{
		path := "/"
		req, err := http.ReadRequest(bufio.NewReader(bytes.NewBufferString("GET " + path + " HTTP/1.1\r\nHost: test\r\n\r\n")))
		if err != nil {
			t.Errorf("%s", err)
		}
		resp := httptest.NewRecorder()
		nodes.ServeHTTP(resp, req)
		if resp.Code != 200 {
			t.Fatal(resp.Code)
		}
	}

	{
		path := "/foo"
		req, err := http.ReadRequest(bufio.NewReader(bytes.NewBufferString("GET " + path + " HTTP/1.1\r\nHost: test\r\n\r\n")))
		if err != nil {
			t.Errorf("%s", err)
		}
		resp := httptest.NewRecorder()
		nodes.ServeHTTP(resp, req)
		if resp.Code != 404 {
			t.Fatal(resp.Code)
		}
	}
}
