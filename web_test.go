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
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

type WebDumbcasAppMock struct {
	*DumbcasAppMock
	socket  net.Listener
	closed  chan bool
	baseUrl string
}

func makeWebDumbcasAppMock(t *testing.T) *WebDumbcasAppMock {
	return &WebDumbcasAppMock{
		DumbcasAppMock: makeDumbcasAppMock(t),
		closed:         make(chan bool),
	}
}

func (f *WebDumbcasAppMock) goWeb() {
	f.Assertf(f.socket == nil, "Socket is empty")
	cmd := FindCommand(f, "web")
	r := cmd.CommandRun().(*webRun)
	r.Root = "\\foo"
	// Simulate -local. It is important to use it while testing otherwise it
	// may trigger the Windows firewall.
	r.local = true
	c := make(chan net.Listener)
	go func() {
		err := r.main(f, c)
		f.log.Printf("Closed: %s", err)
		f.closed <- true
	}()
	f.log.Print("Starting")
	f.socket = <-c
	f.baseUrl = fmt.Sprintf("http://%s", f.socket.Addr().String())
	f.log.Printf("Started at %s", f.baseUrl)
}

func (f *WebDumbcasAppMock) closeWeb() {
	f.socket.Close()
	f.socket = nil
	f.baseUrl = ""
	<-f.closed
	f.CheckBuffer(false, false)
}

func (f *WebDumbcasAppMock) get(url string, expectedUrl string) *http.Response {
	r, err := http.Get(f.baseUrl + url)
	f.Assertf(err == nil, "Oops: %s", err)
	f.Assertf(expectedUrl == "" || r.Request.URL.Path == expectedUrl, "%s != %s", expectedUrl, r.Request.URL.Path)
	return r
}

func (f *WebDumbcasAppMock) get404(url string) {
	r, err := http.Get(f.baseUrl + url)
	f.Assertf(err == nil, "Oops: %s", err)
	f.Assertf(r.StatusCode == 404, "Expected 404, got %d. %s", r.StatusCode, r.Body)
}

func readBody(t *TB, r *http.Response) string {
	bytes, err := ioutil.ReadAll(r.Body)
	t.Assertf(err == nil, "Oops: %s", err)
	r.Body.Close()
	return string(bytes)
}

func expectedBody(t *TB, r *http.Response, expected string) {
	actual := readBody(t, r)
	t.Assertf(actual == expected, "%v != %v", expected, actual)
}

func TestWeb(t *testing.T) {
	t.Parallel()
	f := makeWebDumbcasAppMock(t)
	cmd := FindCommand(f, "web")
	f.Assertf(cmd != nil, "Failed to find 'web'")
	run := cmd.CommandRun().(*webRun)
	// Sets -root to an invalid non-empty string.
	run.Root = "\\test_web"

	// Create a tree of stuff. Call the factory functions directly because we
	// can't use Run(). The reason Run() can't be used is because we need the
	// channel to get the socket address back.
	f.DumbcasAppMock.MakeCasTable("")
	f.DumbcasAppMock.LoadNodesTable("", f.cas)
	tree1 := map[string]string{
		"file1":           "content1",
		"dir1/dir2/file2": "content2",
	}
	sha1tree, nodeName, sha1 := archiveData(f.TB, f.cas, f.nodes, tree1)
	nodeName = strings.Replace(nodeName, string(filepath.Separator), "/", -1)

	f.log.Print("T: Serve over web and verify files are accessible.")
	f.goWeb()
	f.log.Print("T: Make sure it gets a redirect.", sha1, nodeName)
	r := f.get("/content/retrieve/nodes", "/content/retrieve/nodes/")
	month := time.Now().UTC().Format("2006-01")
	expected := fmt.Sprintf("<html><body><pre><a href=\"%s/\">%s/</a>\n<a href=\"tags/\">tags/</a>\n</pre></body></html>", month, month)
	expectedBody(f.TB, r, expected)
	f.log.Print("T: Get the directory.")
	r = f.get("/content/retrieve/nodes/"+month, "/content/retrieve/nodes/"+month+"/")
	actual := readBody(f.TB, r)
	re := regexp.MustCompile("\\\"(.*)\\\"")
	nodeItems := re.FindStringSubmatch(actual)
	f.Assertf(len(nodeItems) == 2, "%s", actual)
	f.Assertf(month+"/"+nodeItems[1] == nodeName, "Unexpected grep: %s", nodeName)

	f.log.Print("T: Get the node.")
	r = f.get("/content/retrieve/nodes/"+nodeName, "/content/retrieve/nodes/"+nodeName+"/")
	expected = "<html><body><pre><a href=\"dir1/\">dir1/</a>\n<a href=\"file1\">file1</a>\n</pre></body></html>"
	expectedBody(f.TB, r, expected)

	r = f.get("/content/retrieve/default/"+sha1tree["file1"], "/content/retrieve/default/"+sha1tree["file1"])
	expectedBody(f.TB, r, "content1")
	r = f.get("/content/retrieve/nodes/"+nodeName+"/file1", "")
	expectedBody(f.TB, r, "content1")
	r = f.get("/content/retrieve/nodes/"+nodeName+"/dir1/dir2/file2", "")
	expectedBody(f.TB, r, "content2")

	f.closeWeb()
}
