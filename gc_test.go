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
	"testing"
)

// Returns a sorted list of all the entries.
func EnumerateAsList(t *testing.T, cas CasTable) []string {
	items := []string{}
	for v := range cas.Enumerate() {
		if v.Error != nil {
			t.Fatal("Unexpected failure")
		}
		// Hardcoded for sha1.
		if len(v.Item) != 40 {
			t.Fatal("Unexpected empty entry")
		}
		items = append(items, v.Item)
	}
	return items
}

func TestGc(t *testing.T) {
	t.Parallel()
	f := makeDumbcasAppMock(t)

	args := []string{"gc", "-root=\\"}
	f.Run(args, 0)

	if items := EnumerateAsList(f.T, f.cas); len(items) != 0 {
		t.Fatalf("Unexpected items: %s", items)
	}
	// Create a tree of stuff.
	tree1 := map[string]string{
		"file1":           "content1",
		"dir1/dir2/file2": "content2",
	}
	archiveData(t, f.cas, f.nodes, tree1)

	args = []string{"gc", "-root=\\"}
	f.Run(args, 0)
	if items := EnumerateAsList(f.T, f.cas); len(items) != 3 {
		t.Fatalf("Unexpected items: %s", items)
	}

}
