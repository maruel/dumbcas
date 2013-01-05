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

func TestGc(t *testing.T) {
	t.Parallel()
	f := makeDumbcasAppMock(t)

	args := []string{"gc", "-root=\\"}
	f.Run(args, 0)

	items := EnumerateCasAsList(f.TB, f.cas)
	f.Assertf(len(items) == 0, "Unexpected items: %s", items)

	// Create a tree of stuff.
	tree := map[string]string{
		"file1":           "content1",
		"dir1/dir2/file2": "content2",
	}
	archiveData(f.TB, f.cas, f.nodes, tree)

	args = []string{"gc", "-root=\\"}
	f.Run(args, 0)
	items = EnumerateCasAsList(f.TB, f.cas)
	f.Assertf(len(items) == 3, "Unexpected items: %q", items)
	n1 := EnumerateNodesAsList(f.TB, f.nodes)
	f.Assertf(len(n1) == 2, "Unexpected items: %q", n1)

	// Add anothera tree of stuff.
	tree = map[string]string{
		"file3":           "content3",
		"dir1/dir4/file5": "content5",
	}
	archiveData(f.TB, f.cas, f.nodes, tree)

	items = EnumerateCasAsList(f.TB, f.cas)
	f.Assertf(len(items) == 6, "Unexpected items: %s", items)
	n2 := EnumerateNodesAsList(f.TB, f.nodes)
	f.Assertf(len(n2) == 3, "Unexpected items: %q", n2)
	// TODO(maruel): nodes.Remove(n1[0])
}
