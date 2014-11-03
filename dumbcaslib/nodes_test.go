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
	"bufio"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maruel/ut"
)

func TestFakeNodesTable(t *testing.T) {
	t.Parallel()
	cas := MakeMemoryCasTable()
	testNodesTableImpl(t, cas, MakeMemoryNodesTable(cas))
}

func request(t testing.TB, nodes NodesTable, path string, expectedCode int, expectedBody string) string {
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewBufferString("GET " + path + " HTTP/1.1\r\nHost: test\r\n\r\n")))
	ut.AssertEqual(t, nil, err)

	resp := httptest.NewRecorder()
	nodes.ServeHTTP(resp, req)
	bytes, err := ioutil.ReadAll(resp.Body)
	ut.AssertEqual(t, nil, err)

	body := string(bytes)
	ut.AssertEqual(t, expectedCode, resp.Code)
	ut.AssertEqualf(t, true, expectedBody == "" || body == expectedBody, "%s: %#s != %#s", path, expectedBody, body)
	return body
}

// marshalData returns the tree of sha1s and the json encoded Node as bytes.
func marshalData(t testing.TB, tree map[string]string) (map[string]string, []byte) {
	sha1tree := map[string]string{}
	entries := &Entry{}
	for k, v := range tree {
		h := Sha1Bytes([]byte(v))
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
	ut.AssertEqual(t, nil, err)
	return sha1tree, data
}

// archiveData archives a tree fictious data.
// Returns (tree of sha1s, name of the node, sha1 of the node entry).
// Accept the paths as posix.
func archiveData(t testing.TB, cas CasTable, nodes NodesTable, tree map[string]string) (map[string]string, string, string) {
	sha1tree, entries := marshalData(t, tree)
	for k, v := range tree {
		err := cas.AddEntry(bytes.NewBuffer([]byte(v)), sha1tree[k])
		ut.AssertEqualf(t, true, err == nil || err == os.ErrExist, "Unexpected error: %s", err)
	}
	entrySha1, err := AddBytes(cas, entries)
	ut.AssertEqual(t, nil, err)

	// And finally add the node.
	now := time.Now().UTC()
	nodeName, err := nodes.AddEntry(&Node{entrySha1, "useful comment"}, "fictious")
	ut.AssertEqual(t, nil, err)
	ut.AssertEqualf(t, true, strings.HasPrefix(nodeName, now.Format("2006-01")+string(filepath.Separator)), "Invalid node name %s", nodeName)
	return sha1tree, nodeName, entrySha1
}

func testNodesTableImpl(t testing.TB, cas CasTable, nodes NodesTable) {
	items, err := EnumerateNodesAsList(nodes)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, []string{}, items)

	tree1 := map[string]string{
		"file1":           "content1",
		"dir1/dir2/file2": "content2",
	}
	archiveData(t, cas, nodes, tree1)
	items, err = EnumerateNodesAsList(nodes)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, 2, len(items))
	name := strings.Replace(items[0], string(filepath.Separator), "/", -1)

	body := request(t, nodes, "/", 200, "")
	ut.AssertEqual(t, 2, strings.Count(body, "<a "))
	request(t, nodes, "/foo", 404, "")
	request(t, nodes, "/foo/", 404, "")
	request(t, nodes, "/"+name, 301, "")
	request(t, nodes, "/"+name+"/", 200, "")
	request(t, nodes, "/"+name+"/file1", 200, "content1")
	request(t, nodes, "/"+name+"/dir1/dir2/file2", 200, "content2")
	request(t, nodes, "/"+name+"/dir1/dir2/file3", 404, "")
	request(t, nodes, "/"+name+"/dir1/dir2", 301, "")
}
