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

func TestInfo(t *testing.T) {
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

	args := []string{"info", "-root=\\test_archive", nodeName}
	f.Run(args, 0)

	expected := " dir1/bar(4)\n dir1/dir2/dir3/foo(4)\n dir1/dir2/file2(8)\n file1(8)\n x(2)\nTotal 5\n"
	f.CheckOut(expected)
	f.CheckBuffer(false, false)
}
