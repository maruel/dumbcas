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

	"github.com/maruel/ut"
)

func TestFsckEmpty(t *testing.T) {
	t.Parallel()
	f := makeDumbcasAppMock(t)
	args := []string{"fsck", "-root=\\test_fsck_empty"}
	f.Run(args, 0)
	items := EnumerateCasAsList(f.TB, f.cas)
	ut.AssertEqual(t, []string{}, items)
	nodes := EnumerateNodesAsList(f.TB, f.nodes)
	ut.AssertEqual(t, []string{}, nodes)
}

func TestFsckCorruptCasFile(t *testing.T) {
	t.Parallel()
	f := makeDumbcasAppMock(t)
	args := []string{"fsck", "-root=\\test_fsck_cas"}
	f.Run(args, 0)

	archiveData(f.TB, f.cas, f.nodes, map[string]string{
		"file1":           "content1",
		"dir1/dir2/file2": "content2",
	})

	// Corrupt an item in CasTable.
	cas := f.cas.(*fakeCasTable)
	cas.entries[sha1String("content1")] = []byte("content5")
	f.Run(args, 0)

	// One entry disapeared. I hope you had a valid secondary copy of your
	// CasTable.
	i1 := EnumerateCasAsList(f.TB, f.cas)
	ut.AssertEqual(t, 2, len(i1))

	// Note: The node is not quarantined, because in theory the data could be
	// found on another copy of the CasTable so it's preferable to not delete the
	// node.
	n1 := EnumerateNodesAsList(f.TB, f.nodes)
	ut.AssertEqual(t, 2, len(n1))
}

func TestFsckCorruptNodeEntry(t *testing.T) {
	t.Parallel()
	f := makeDumbcasAppMock(t)
	args := []string{"fsck", "-root=\\test_fsck_corrupt"}
	f.Run(args, 0)

	// Create a tree of stuff.
	archiveData(f.TB, f.cas, f.nodes, map[string]string{
		"file1":           "content1",
		"dir1/dir2/file2": "content2",
	})

	// Corrupt an item in NodesTable.
	nodes := f.nodes.(*fakeNodesTable)
	nodes.entries["tags/fictious"] = []byte("Invalid Json")
	f.Run(args, 0)

	i1 := EnumerateCasAsList(f.TB, f.cas)
	ut.AssertEqual(t, 3, len(i1))
	n1 := EnumerateNodesAsList(f.TB, f.nodes)
	ut.AssertEqual(t, 1, len(n1))
}
