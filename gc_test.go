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
	"sort"
	"testing"

	"github.com/maruel/ut"
)

func TestGcEmpty(t *testing.T) {
	t.Parallel()
	f := makeDumbcasAppMock(t)
	args := []string{"gc", "-root=\\test_gc_empty"}
	f.Run(args, 0)
	i := EnumerateCasAsList(f.TB, f.cas)
	ut.AssertEqual(t, []string{}, i)
}

func TestGcKept(t *testing.T) {
	t.Parallel()
	f := makeDumbcasAppMock(t)
	args := []string{"gc", "-root=\\test_gc_kept"}
	f.Run(args, 0) // Instantiate f.cas and f.nodes

	// Create a tree of stuff.
	archiveData(f.TB, f.cas, f.nodes, map[string]string{
		"file1":           "content1",
		"dir1/dir2/file2": "content2",
	})

	i1 := EnumerateCasAsList(f.TB, f.cas)
	ut.AssertEqual(t, 3, len(i1))
	n1 := EnumerateNodesAsList(f.TB, f.nodes)
	ut.AssertEqual(t, 2, len(n1))

	f.Run(args, 0)

	// Nothing disapeared.
	i2 := EnumerateCasAsList(f.TB, f.cas)
	ut.AssertEqual(t, i1, i2)
	n2 := EnumerateNodesAsList(f.TB, f.nodes)
	ut.AssertEqual(t, n1, n2)
}

func TestGcTrim(t *testing.T) {
	t.Parallel()
	f := makeDumbcasAppMock(t)
	args := []string{"gc", "-root=\\test_gc_trim"}
	f.Run(args, 0) // Instantiate f.cas and f.nodes

	// Create a tree of stuff.
	archiveData(f.TB, f.cas, f.nodes, map[string]string{
		"file1":           "content1",
		"dir1/dir2/file2": "content2",
	})
	i1 := EnumerateCasAsList(f.TB, f.cas)
	n1 := EnumerateNodesAsList(f.TB, f.nodes)

	// Add anothera tree of stuff.
	archiveData(f.TB, f.cas, f.nodes, map[string]string{
		"file3":           "content3",
		"dir1/dir4/file5": "content4",
		"dir6/file7":      "content5",
		"file1a":          "content1",
	})

	i2 := EnumerateCasAsList(f.TB, f.cas)
	ut.AssertEqual(t, 7, len(i2))
	n2 := EnumerateNodesAsList(f.TB, f.nodes)
	ut.AssertEqual(t, 3, len(n2))

	// Remove the first node and gc.
	err := f.nodes.Remove(n1[0])
	ut.AssertEqual(t, nil, err)
	f.Run(args, 0)
	i3 := EnumerateCasAsList(f.TB, f.cas)
	ut.AssertEqual(t, 5, len(i3))
	n3 := EnumerateNodesAsList(f.TB, f.nodes)
	ut.AssertEqual(t, 2, len(n3))

	// Check both: "n3 == n2 - n1[0]" and "i3 == i2 - i1 + sha1(content1)"
	rest := Sub(n2, []string{n1[0]})
	ut.AssertEqual(t, n3, rest)
	rest = Sub(i2, i1)
	rest = append(rest, sha1String("content1"))
	sort.Strings(rest)
	ut.AssertEqual(t, i3, rest)
}
