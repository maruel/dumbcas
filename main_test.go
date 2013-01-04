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
	"log"
	"net"
	"testing"
)

type DumbcasAppMock struct {
	ApplicationMock
	log *log.Logger
	// Statefullness
	cache *mockCache
	cas   CasTable
	nodes NodesTable
	// Optional stuff
	socket  net.Listener
	closed  chan bool
	baseUrl string
}

func (a *DumbcasAppMock) GetLog() *log.Logger {
	return a.log
}

func (a *DumbcasAppMock) Run(args []string, expected int) {
	a.GetLog().Printf("%s", args)
	if returncode := Run(a, args); returncode != expected {
		a.Fatal("Unexpected return code", returncode)
	}
}

func makeDumbcasAppMock(t *testing.T, verbose bool) *DumbcasAppMock {
	a := &DumbcasAppMock{
		ApplicationMock: *makeAppMock(t),
		log:             getLog(verbose),
		closed:          make(chan bool),
	}
	return a
}

func sha1Map(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = sha1String(v)
	}
	return out
}

/*
func runarchive(f *DumbcasAppMock) {
	args := []string{"archive", "-root=" + f.tempArchive, path.Join(f.tempData, "toArchive")}
	f.Run(args, 0)
	f.checkBuffer(true, false)
}

func TestSmoke(t *testing.T) {
	// End-to-end smoke test that tests archive, web, gc and fsck.
	t.Parallel()
	f:=makeDumbcasMock(t, false)
	f.makeDirs()
	defer f.cleanup()

	// Create a tree of stuff.
	tree := map[string]string{
		"toArchive":          "x\ndir1\n",
		"x":                  "x\n",
		"dir1/bar":           "bar\n",
		"dir1/dir2/dir3/foo": "foo\n",
	}
	if err := createTree(f.tempData, tree); err != nil {
		f.Fatal(err)
	}

	log.Print("T: Archive.")
	runarchive(f)
	log.Print("T: Serve over web and verify files are accessible.")
	f.goWeb()
	// Make sure it gets a redirect.
	r := f.get("/content/retrieve/nodes", "/content/retrieve/nodes/")
	month := time.Now().UTC().Format("2006-01")
	expected := fmt.Sprintf("<html><body><pre><a href=\"%s/\">%s/</a>\n<a href=\"tags/\">tags/</a>\n</pre></body></html>", month, month)
	expectedBody(t, r, expected)
	r = f.get("/content/retrieve/nodes/"+month, "/content/retrieve/nodes/"+month+"/")
	actual := readBody(t, r)
	re := regexp.MustCompile("\\\"(.*)\\\"")
	nodeItems := re.FindStringSubmatch(actual)
	if len(nodeItems) != 2 {
		t.Fatal(actual)
	}
	nodeName := nodeItems[1]
	r = f.get("/content/retrieve/nodes/"+month+"/"+nodeName, "/content/retrieve/nodes/"+month+"/"+nodeName+"/")
	expected = "<html><body><pre><a href=\"tmp/\">tmp/</a>\n</pre></body></html>"
	expectedBody(t, r, expected)

	sha1 := sha1String(tree["dir1/dir2/dir3/foo"])
	r = f.get("/content/retrieve/default/"+sha1, "/content/retrieve/default/"+sha1)
	expectedBody(t, r, tree["dir1/dir2/dir3/foo"])
	r = f.get("/content/retrieve/nodes/"+month+"/"+nodeName+f.tempData+"/dir1/dir2/dir3/foo", "")
	expectedBody(t, r, tree["dir1/dir2/dir3/foo"])
	r = f.get("/content/retrieve/nodes/"+month+"/"+nodeName+f.tempData+"/dir1/bar", "")
	expectedBody(t, r, tree["dir1/bar"])

	f.closeWeb()

	log.Print("T: Remove dir1/dir2/dir3/foo, archive again and gc.")
	if err := os.Remove(path.Join(f.tempData, "dir1", "dir2", "dir3", "foo")); err != nil {
		f.Fatal(err)
	}
	runarchive(f)
	args := []string{"gc", "-root=" + f.tempArchive}
	f.Run(args, 0)
	f.checkBuffer(false, false)
	log.Print("T: Lookup dir1/dir2/dir3/foo is still present in the backup")
	f.goWeb()
	r = f.get("/content/retrieve/nodes/"+month+"/"+nodeName+f.tempData+"/dir1/dir2/dir3/foo", "")
	expectedBody(t, r, tree["dir1/dir2/dir3/foo"])
	r = f.get("/content/retrieve/nodes/"+month+"/"+nodeName+f.tempData+"/dir1/bar", "")
	expectedBody(t, r, tree["dir1/bar"])
	f.closeWeb()

	log.Print("T: Remove the node, gc and lookup with web the file is not present anymore.")
	if err := os.Remove(path.Join(f.tempArchive, "nodes", month, nodeName)); err != nil {
		f.Fatal(err)
	}
	matches, err := readDirNames(path.Join(f.tempArchive, "nodes", month))
	if err != nil {
		f.Fatal(err)
	}
	if len(matches) != 1 {
		f.Fatal(matches)
	}
	nodeName = matches[0]
	args = []string{"gc", "-root=" + f.tempArchive}
	f.Run(args, 0)
	f.checkBuffer(false, false)
	f.goWeb()
	f.get404("/content/retrieve/nodes/" + month + "/" + nodeName + f.tempData + "/dir1/dir2/dir3/foo")
	r = f.get("/content/retrieve/nodes/"+month+"/"+nodeName+f.tempData+"/dir1/bar", "")
	expectedBody(t, r, tree["dir1/bar"])
	f.closeWeb()

	log.Print("T: Corrupt and fsck.")
	sha1 = sha1String(tree["dir1/bar"])
	file, err := os.OpenFile(path.Join(f.tempArchive, "cas", sha1[:3], sha1[3:]), os.O_WRONLY, 0)
	if err != nil {
		f.Fatal("File is missing", err)
	}
	if _, err := io.WriteString(file, "something else"); err != nil {
		f.Fatal("Write fail", err)
	}
	file.Sync()
	file.Close()
	args = []string{"fsck", "-root=" + f.tempArchive}
	f.Run(args, 0)
	f.checkBuffer(false, false)
	log.Print("T: Verify dir1/bar was removed.")
	file, err = os.OpenFile(path.Join(f.tempArchive, "cas", sha1[:3], sha1[3:]), os.O_WRONLY, 0)
	if err == nil {
		f.Fatal("File was not moved out")
	}
	// Lookup with web the file is not present anymore.
	f.goWeb()
	f.get404("/content/retrieve/nodes/" + month + "/" + nodeName + f.tempData + "/dir1/bar")
	f.closeWeb()
}*/
