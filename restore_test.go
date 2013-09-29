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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// Reads all files in the tree and return their content as a map.
func ReadTree(root string) (map[string]string, error) {
	out := map[string]string{}
	visit := func(path string, fi os.FileInfo, err error) error {
		if fi.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		b, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(root, path)
		out[relPath] = string(b)
		return nil
	}
	err := filepath.Walk(root, visit)
	return out, err
}

func MapsEquals(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		v, ok := b[k]
		if !ok || a[k] != v {
			return false
		}
	}
	return true
}

func TestRestore(t *testing.T) {
	t.Parallel()
	f := makeDumbcasAppMock(t)
	// Force the creation of CAS and NodesTable so content can be archived in
	// memory before running the command.
	f.MakeCasTable("")
	f.LoadNodesTable("", f.cas)

	// Create an archive.
	tree := map[string]string{
		"dir1/bar":           "bar\n",
		"dir1/dir2/dir3/foo": "foo\n",
		"dir1/dir2/file2":    "content2",
		"file1":              "content1",
		"x":                  "x\n",
	}
	_, nodeName, _ := archiveData(f.TB, f.cas, f.nodes, tree)

	tempData := makeTempDir(f.TB, "restore")
	defer removeTempDir(tempData)

	args := []string{"restore", "-root=\\test_archive", "-out=" + tempData, nodeName}
	f.Run(args, 0)
	f.CheckBuffer(true, false)

	actualTree, err := ReadTree(tempData)
	if err != nil {
		t.Fatalf("Failed to read tree %s: %s", tempData, err)
	}
	if !MapsEquals(tree, actualTree) {
		t.Fatalf("Tree mismatch: %v != %v", tree, actualTree)
	}
}
