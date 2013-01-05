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

func TestFsckEmpty(t *testing.T) {
	t.Parallel()
	f := makeDumbcasAppMock(t)
	args := []string{"fsck", "-root=\\foo_bar"}
	f.Run(args, 0)
	items := EnumerateCasAsList(f.TB, f.cas)
	f.Assertf(len(items) == 0, "Unexpected items: %s", items)
	nodes := EnumerateNodesAsList(f.TB, f.nodes)
	f.Assertf(len(nodes) == 0, "Unexpected nodes: %q", nodes)
}

func TestFsckCorruptCasFile(t *testing.T) {
	t.Parallel()
	f := makeDumbcasAppMock(t)
	args := []string{"fsck", "-root=\\foo_bar"}
	f.Run(args, 0)

	archiveData(f.TB, f.cas, f.nodes, map[string]string{
		"file1":           "content1",
		"dir1/dir2/file2": "content2",
	})

	// Corrupt an item in CasTable.
	casMock := f.cas.(*mockCasTable)
	casMock.entries[sha1String("content1")] = []byte("content5")
	f.Run(args, 0)

	// One entry disapeared. I hope you had a valid secondary copy of your
	// CasTable.
	i1 := EnumerateCasAsList(f.TB, f.cas)
	f.Assertf(len(i1) == 2, "Unexpected items: %d", len(i1))

	// Note: The node is not quarantined, because in theory the data could be
	// found on another copy of the CasTable so it's preferable to not delete the
	// node.
	n1 := EnumerateNodesAsList(f.TB, f.nodes)
	f.Assertf(len(n1) == 2, "Unexpected nodes: %q", n1)
}

func TestFsckCorruptNodeEntry(t *testing.T) {
	t.Parallel()
	f := makeDumbcasAppMock(t)
	args := []string{"fsck", "-root=\\foo_bar"}
	f.Run(args, 0)

	// Create a tree of stuff.
	archiveData(f.TB, f.cas, f.nodes, map[string]string{
		"file1":           "content1",
		"dir1/dir2/file2": "content2",
	})

	// Corrupt an item in NodesTable.
	nodesMock := f.nodes.(*mockNodesTable)
	nodesMock.entries["tags/fictious"] = Node{}
	f.Run(args, 0)

	i1 := EnumerateCasAsList(f.TB, f.cas)
	f.Assertf(len(i1) == 3, "Unexpected items: %d", len(i1))
	n1 := EnumerateNodesAsList(f.TB, f.nodes)
	f.Assertf(len(n1) == 1, "Unexpected nodes: %q", n1)
}
