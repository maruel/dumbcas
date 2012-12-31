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
	"bytes"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path"
	"regexp"
	"testing"
	"time"
)

// Logging is a global object so it can't be checked for when tests are run in parallel.
var bufLog bytes.Buffer

var enableOutput = false

func init() {
	// Reduces output. Comment out to get more logs.
	if !enableOutput {
		log.SetOutput(&bufLog)
	}
	log.SetFlags(log.Lmicroseconds)
}

func GetRandRune() rune {
	chars := "0123456789abcdefghijklmnopqrstuvwxyz"
	lengthBig := big.NewInt(int64(len(chars)))
	val, err := rand.Int(rand.Reader, lengthBig)
	if err != nil {
		panic("Rand failed")
	}
	return rune(chars[int(val.Int64())])
}

// Creates a temporary directory.
func makeTempDir(name string) (string, error) {
	prefix := "dumbcas_" + name + "_"
	length := 8
	tempDir := os.TempDir()

	ranPath := make([]rune, length)
	for i := 0; i < length; i++ {
		ranPath[i] = GetRandRune()
	}
	tempFull := path.Join(tempDir, prefix+string(ranPath))
	for {
		err := os.Mkdir(tempFull, 0700)
		if os.IsExist(err) {
			// Add another random character.
			ranPath = append(ranPath, GetRandRune())
		}
		return tempFull, nil
	}
	return "", errors.New("Internal error")
}

func removeTempDir(tempDir string) {
	if err := os.RemoveAll(tempDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to clean up %s", tempDir)
	}
}

func createTree(rootDir string, tree map[string]string) error {
	for relPath, content := range tree {
		base := path.Dir(relPath)
		if base != "." {
			if err := os.MkdirAll(path.Join(rootDir, base), 0700); err != nil && !os.IsExist(err) {
				return err
			}
		}
		f, err := os.Create(path.Join(rootDir, relPath))
		if err != nil {
			return err
		}
		f.WriteString(content)
		f.Sync()
		f.Close()
	}
	return nil
}

type ApplicationMock struct {
	DefaultApplication
	*testing.T
	bufOut bytes.Buffer
	bufErr bytes.Buffer
	bufLog bytes.Buffer
	log    *log.Logger
	// Optional stuff
	tempArchive string
	tempData    string
	socket      net.Listener
	closed      chan bool
	baseUrl     string
}

func (a *ApplicationMock) GetOut() io.Writer {
	return &a.bufOut
}

func (a *ApplicationMock) GetErr() io.Writer {
	return &a.bufErr
}

func (a *ApplicationMock) GetLog() *log.Logger {
	return a.log
}

func (a *ApplicationMock) Run(args []string, expected int) {
	a.GetLog().Printf("%s", args)
	if returncode := Run(a, args); returncode != expected {
		a.Fatal("Unexpected return code", returncode)
	}
}

type CommandMock struct {
	Command
	flags flag.FlagSet
}

func (c *CommandMock) GetFlags() *flag.FlagSet {
	return &c.flags
}

func makeMock(t *testing.T) *ApplicationMock {
	a := &ApplicationMock{
		DefaultApplication: application,
		testing.T:          t,
		closed:             make(chan bool),
	}
	if !enableOutput {
		// Send the log to the test-specific buffer.
		a.log = log.New(&a.bufLog, "", log.Lmicroseconds)
	} else {
		// Send directly to output for test debugging.
		a.log = log.New(os.Stderr, "", log.Lmicroseconds)
	}
	// Uncomment to send the log to the general buffer.
	//a.log = log.New(&bufLog, "", log.Lmicroseconds)
	for i, c := range a.Commands {
		a.Commands[i] = &CommandMock{c, *c.GetFlags()}
	}
	return a
}

func baseInit(t *testing.T) *ApplicationMock {
	// The test cases in this file are multi-thread safe. Comment out to ease
	// debugging.
	t.Parallel()

	return makeMock(t)
}

type tempStuff struct {
}

func (f *ApplicationMock) checkBuffer(out, err bool) {
	if out {
		/*
			if f.bufOut.Len() == 0 {
				// Print Stderr to see what happened.
				f.Fatal("Expected buffer; " + f.bufErr.String())
			}
		*/
	} else {
		if f.bufOut.Len() != 0 {
			f.Fatalf("Unexpected buffer:\n%s", f.bufOut.String())
		}
	}

	if err {
		if f.bufErr.Len() == 0 {
			f.Fatal("Expected buffer; " + f.bufOut.String())
		}
	} else {
		if f.bufErr.Len() != 0 {
			f.Fatal("Unexpected buffer: " + f.bufErr.String())
		}
	}
	f.bufOut.Reset()
	f.bufErr.Reset()
}

func (f *ApplicationMock) makeDirs() {
	tempData, err := makeTempDir("data")
	if err != nil {
		f.Fatalf("Failed to create data dir: %s", err)
	} else {
		f.tempData = tempData
	}
	tempArchive, err := makeTempDir("out")
	if err != nil {
		f.Fatalf("Failed to create archive dir: %s", err)
	} else {
		f.tempArchive = tempArchive
	}
}
func (f *ApplicationMock) cleanup() {
	if f.tempArchive != "" {
		removeTempDir(f.tempArchive)
	}
	if f.tempData != "" {
		removeTempDir(f.tempData)
	}
}

func (f *ApplicationMock) goWeb() {
	if f.socket != nil {
		f.Fatal("Socket is empty")
	}
	c := make(chan net.Listener)
	go func() {
		webMain(f, 0, c)
		f.closed <- true
	}()
	f.socket = <-c
	f.baseUrl = fmt.Sprintf("http://%s", f.socket.Addr().String())
}

func (f *ApplicationMock) closeWeb() {
	f.socket.Close()
	f.socket = nil
	f.baseUrl = ""
	<-f.closed
	f.checkBuffer(false, false)
}

func (f *ApplicationMock) get(url string, expectedUrl string) *http.Response {
	r, err := http.Get(f.baseUrl + url)
	if err != nil {
		f.Fatal(err)
	}
	if expectedUrl != "" && r.Request.URL.Path != expectedUrl {
		f.Fatalf("%s != %s", expectedUrl, r.Request.URL.Path)
	}
	return r
}

func (f *ApplicationMock) get404(url string) {
	r, err := http.Get(f.baseUrl + url)
	if err != nil {
		f.Fatal(err)
	}
	if r.StatusCode != 404 {
		f.Fatal(r.StatusCode, r.Body)
	}
}

func TestHelp(t *testing.T) {
	f := baseInit(t)
	args := []string{"help"}
	f.Run(args, 0)
	f.checkBuffer(true, false)
}

func TestBadFlag(t *testing.T) {
	f := baseInit(t)
	args := []string{"archive", "-random"}
	f.Run(args, 0)
	// Prints to Stderr.
	f.checkBuffer(false, true)
}

func readBody(t *testing.T, r *http.Response) string {
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	return string(bytes)
}

func expectedBody(t *testing.T, r *http.Response, expected string) {
	actual := readBody(t, r)
	if actual != expected {
		t.Fatalf("%v != %v", expected, actual)
	}
}

func sha1Map(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = sha1String(v)
	}
	return out
}

func runarchive(f *ApplicationMock) {
	args := []string{"archive", "-root=" + f.tempArchive, path.Join(f.tempData, "toArchive")}
	f.Run(args, 0)
	f.checkBuffer(true, false)
}

func TestSmoke(t *testing.T) {
	// End-to-end smoke test that tests archive, web, gc and fsck.
	f := baseInit(t)
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
}
