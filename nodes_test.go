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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"sort"
	"strings"
	"testing"
	"time"
)

type mockNodesTable struct {
	entries map[string][]byte
	cas     CasTable
	t       *TB
}

func (a *DumbcasAppMock) LoadNodesTable(rootDir string, cas CasTable) (NodesTable, error) {
	//return loadNodesTable(rootDir, cas, a.GetLog())
	if a.nodes == nil {
		a.nodes = &mockNodesTable{make(map[string][]byte), a.cas, a.TB}
	}
	return a.nodes, nil
}

func (m *mockNodesTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.t.log.Printf("mockNodesTable.ServeHTTP(%s)", r.URL.Path)
	suburl := r.URL.Path[1:]
	if suburl != "" {
		// Slow search, it's fine for a mock.
		for k, v := range m.entries {
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
				entryFs := EntryFileSystem{cas: m.cas, entry: entry}
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
	for k, _ := range m.entries {
		if strings.HasPrefix(k, suburl) {
			v := strings.SplitAfterN(k[len(suburl):], "/", 2)
			items = append(items, v[0])
		}
	}
	if len(items) != 0 {
		if needRedirect {
			// Not strictly valid but fine enough for a mock.
			// TODO(maruel): posix-specific.
			localRedirect(w, r, path.Base(r.URL.Path)+"/")
			return
		}
		dirList(w, items)
		return
	}
	http.Error(w, "Yo dawg", http.StatusNotFound)
}

func (m *mockNodesTable) AddEntry(node *Node, name string) (string, error) {
	m.t.log.Printf("mockNodesTable.AddEntry(%s)", name)
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
		nodePath = path.Join(monthName, nodeName)
		if _, ok := m.entries[nodePath]; !ok {
			m.entries[nodePath] = data
			break
		}
		// Try ad nauseam.
		suffix += 1
	}
	// The real implementation creates a symlink if possible.
	m.entries[tagsName+"/"+name] = data
	return nodePath, nil
}

func (m *mockNodesTable) Enumerate() <-chan NodeEntry {
	m.t.log.Printf("mockNodesTable.Enumerate() %d", len(m.entries))
	c := make(chan NodeEntry)
	go func() {
		// TODO(maruel): Will blow up if mutated concurrently.
		for k, _ := range m.entries {
			c <- NodeEntry{Item: k}
		}
		close(c)
	}()
	return c
}

func (m *mockNodesTable) Open(item string) (ReadSeekCloser, error) {
	m.t.log.Printf("mockNodesTable.Open(%s)", item)
	data, ok := m.entries[item]
	if !ok {
		return nil, fmt.Errorf("Missing: %s", item)
	}
	return Buffer{bytes.NewReader(data)}, nil
}

func (m *mockNodesTable) Remove(name string) error {
	if _, ok := m.entries[name]; !ok {
		return os.ErrNotExist
	}
	delete(m.entries, name)
	return nil
}

// Returns a sorted list of all the entries.
func EnumerateNodesAsList(t *TB, nodes NodesTable) []string {
	items := []string{}
	for v := range nodes.Enumerate() {
		t.Assertf(v.Error == nil, "Unexpected failure")
		items = append(items, v.Item)
	}
	sort.Strings(items)
	return items
}

func TestNodesTable(t *testing.T) {
	t.Parallel()
	tb := MakeTB(t)
	tempData := makeTempDir(tb, "nodes")
	defer removeTempDir(tempData)

	cas := &mockCasTable{make(map[string][]byte), false, tb}
	nodes, err := loadNodesTable(tempData, cas, tb.log)
	tb.Assertf(err == nil, "Unexpected error: %s", err)

	testNodesTableImpl(tb, cas, nodes)
}

func TestNodesTableMock(t *testing.T) {
	t.Parallel()
	tb := MakeTB(t)
	cas := &mockCasTable{make(map[string][]byte), false, tb}
	nodes := &mockNodesTable{make(map[string][]byte), cas, tb}
	testNodesTableImpl(tb, cas, nodes)
}

func request(t *TB, nodes NodesTable, path string, expectedCode int, expectedBody string) string {
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewBufferString("GET " + path + " HTTP/1.1\r\nHost: test\r\n\r\n")))
	t.Assertf(err == nil, "%s: %s", path, err)

	resp := httptest.NewRecorder()
	nodes.ServeHTTP(resp, req)
	bytes, err := ioutil.ReadAll(resp.Body)
	t.Assertf(err == nil, "%s: %s", path, err)

	body := string(bytes)
	t.Assertf(resp.Code == expectedCode, "%s: %d != %d\n%s", path, expectedCode, resp.Code, body)
	t.Assertf(expectedBody == "" || body == expectedBody, "%s: %#s != %#s", path, expectedBody, body)
	return body
}

// Archives a tree fictious data.
// Returns (tree of sha1s, name of the node, sha1 of the node entry).
// Accept the paths as posix.
func archiveData(t *TB, cas CasTable, nodes NodesTable, tree map[string]string) (map[string]string, string, string) {
	sha1tree := map[string]string{}
	entries := &Entry{}
	for k, v := range tree {
		h, err := AddBytes(cas, []byte(v))
		t.Assertf(err == nil || err == os.ErrExist, "Unexpected error: %s", err)
		sha1tree[k] = h

		e := entries
		parts := strings.Split(k, "/")
		for i := 0; i < len(parts)-1; i++ {
			if e.Files == nil {
				e.Files = map[string]*Entry{}
			}
			if e.Files[parts[i]] == nil {
				e.Files[parts[i]] = &Entry{}
			}
			e = e.Files[parts[i]]
		}
		if e.Files == nil {
			e.Files = map[string]*Entry{}
		}
		e.Files[parts[len(parts)-1]] = &Entry{
			Sha1: h,
			Size: int64(len(v)),
		}
	}

	// Then process entries itself.
	data, err := json.Marshal(entries)
	t.Assertf(err == nil, "Oops")
	entrySha1, err := AddBytes(cas, data)
	t.Assertf(err == nil, "Oops")

	// And finally add the node.
	now := time.Now().UTC()
	nodeName, err := nodes.AddEntry(&Node{entrySha1, "useful comment"}, "fictious")
	t.Assertf(err == nil, "Oops")
	t.Assertf(strings.HasPrefix(nodeName, now.Format("2006-01/")), "Invalid node name %s", nodeName)
	return sha1tree, nodeName, entrySha1
}

func testNodesTableImpl(t *TB, cas CasTable, nodes NodesTable) {
	t.Assertf(len(EnumerateNodesAsList(t, nodes)) == 0, "Found unexpected value")

	tree1 := map[string]string{
		"file1":           "content1",
		"dir1/dir2/file2": "content2",
	}
	archiveData(t, cas, nodes, tree1)
	items := EnumerateNodesAsList(t, nodes)
	t.Assertf(len(items) == 2, "Found items: %q", items)
	name := items[0]

	body := request(t, nodes, "/", 200, "")
	t.Assertf(strings.Count(body, "<a ") == 2, "Unexpected output:\n%s", body)
	request(t, nodes, "/foo", 404, "")
	request(t, nodes, "/foo/", 404, "")
	request(t, nodes, "/"+name, 301, "")
	request(t, nodes, "/"+name+"/", 200, "")
	request(t, nodes, "/"+name+"/file1", 200, "content1")
	request(t, nodes, "/"+name+"/dir1/dir2/file2", 200, "content2")
	request(t, nodes, "/"+name+"/dir1/dir2/file3", 404, "")
	request(t, nodes, "/"+name+"/dir1/dir2", 301, "")
}
