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
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
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
	_, ok := m.entries[r.URL.Path[1:]]
	if !ok {
		http.Error(w, "Yo dawg", http.StatusNotFound)
		return
	}
	// Defer to the cas file system.
	//w.Write(item.asJson)
}

func (m *mockNodesTable) AddEntry(node *Node, name string) error {
	m.log.Printf("mockNodesTable.AddEntry(%s)", name)
	m.entries[name] = *node
	/*
		now := time.Now().UTC()
		// Create one directory store per month.
		monthName := now.Format("2006-01")
		monthDir := path.Join(n.nodesDir, monthName)
		if err := os.MkdirAll(monthDir, 0750); err != nil && !os.IsExist(err) {
			return fmt.Errorf("Failed to create %s: %s\n", monthDir, err)
		}

		suffix := 0
		nodePath := ""
		for {
			nodeName := n.hostname + "_" + now.Format("2006-01-02_15-04-05") + "_" + name
			if suffix != 0 {
				nodeName += fmt.Sprintf("(%d)", suffix)
			}
			nodePath = path.Join(monthDir, nodeName)
			f, err := os.OpenFile(nodePath, os.O_WRONLY|os.O_EXCL|os.O_CREATE, 0640)
			if err != nil {
				// Try ad nauseam.
				suffix += 1
			} else {
				if _, err = f.Write(data); err != nil {
					return fmt.Errorf("Failed to write %s: %s", f.Name(), err)
				}
				n.log.Printf("Saved node: %s", path.Join(monthName, nodeName))
				break
			}
		}
	*/
	return nil
}

func (m *mockNodesTable) Enumerate() <-chan NodeEntry {
	m.log.Printf("mockNodesTable.Enumerate() %d", len(m.entries))
	c := make(chan NodeEntry)
	go func() {
		// TODO(maruel): Will blow up if mutated concurrently.
		for k, v := range m.entries {
			// The comment was discarded.
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

/*
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
*/

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
	// TODO(maruel): The real implementation will return 2, the mock will return 1.
	if count == 0 {
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
