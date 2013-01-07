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
	"path"
	"sort"
	"testing"
)

func TestArchive(t *testing.T) {
	t.Parallel()
	f := makeDumbcasAppMock(t)
	tempData := makeTempDir(f.TB, "archive")
	defer removeTempDir(tempData)

	// Create a tree of stuff.
	tree := map[string]string{
		"toArchive":          "x\ndir1\n",
		"x":                  "x\n",
		"dir1/bar":           "bar\n",
		"dir1/dir2/dir3/foo": "foo\n",
	}
	archived := map[string]string{
		"toArchive":     "x\ndir1\n",
		"x":             "x\n",
		"bar":           "bar\n",
		"dir2/dir3/foo": "foo\n",
	}
	if err := createTree(tempData, tree); err != nil {
		f.Fatal(err)
	}

	args := []string{"archive", "-root=\\test_archive", path.Join(tempData, "toArchive")}
	f.Run(args, 0)
	f.CheckBuffer(true, false)
	items := EnumerateCasAsList(f.TB, f.cas)

	expected := make([]string, 0, len(items))
	sha1tree, entries := marshalData(f.TB, archived)
	for _, v := range sha1tree {
		expected = append(expected, v)
	}
	expected = append(expected, sha1Bytes(entries))
	sort.Strings(expected)
	f.Assertf(Equals(items, expected), "Unexpected items:\n%s\n%s", items, expected)

	nodes := EnumerateNodesAsList(f.TB, f.nodes)
	f.Assertf(len(nodes) == 2, "Unexpected nodes: %s", nodes)
}
