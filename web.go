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
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

func loadReaderAsJson(r io.Reader, value interface{}) error {
	data, err := ioutil.ReadAll(r)
	if err == nil {
		return json.Unmarshal(data, &value)
	}
	return err
}

func loadFileAsJson(filepath string, value interface{}) error {
	f, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("loadFileAsJson(%s): %s", filepath, err)
	}
	defer f.Close()
	return loadReaderAsJson(f, value)
}

// Converts an handler to log every HTTP request.
type LoggingHandler struct {
	handler http.Handler
	l       *log.Logger
}

type loggingResponseWriter struct {
	http.ResponseWriter
	length int
	status int
}

func (l *loggingResponseWriter) Write(data []byte) (size int, err error) {
	size, err = l.ResponseWriter.Write(data)
	l.length += size
	return
}

func (l *loggingResponseWriter) WriteHeader(status int) {
	l.ResponseWriter.WriteHeader(status)
	l.status = status
}

func (l *LoggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l_w := &loggingResponseWriter{ResponseWriter: w}
	l.handler.ServeHTTP(l_w, r)
	l.l.Printf("%s - %3d %6db %4s %s",
		r.RemoteAddr,
		l_w.status,
		l_w.length,
		r.Method,
		r.RequestURI)
}

type restricted struct {
	http.Handler
	methods []string
}

// Restricts request to specific methods
func (d restricted) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, method := range d.methods {
		if r.Method == method {
			d.Handler.ServeHTTP(w, r)
			return
		}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.Error(w, "Invalid Method", http.StatusMethodNotAllowed)
	return
}

func Restrict(h http.Handler, m ...string) http.Handler {
	return restricted{h, m}
}

// Reads a directory list and guarantees to return a list.
func readDirFancy(dirPath string) ([]string, error) {
	names := []string{}
	f, err := os.Open(dirPath)
	if err != nil {
		return names, err
	}
	defer f.Close()
	for {
		dirs, err := f.Readdir(1024)
		if err != nil || len(dirs) == 0 {
			break
		}
		for _, d := range dirs {
			name := d.Name()
			if d.IsDir() {
				name += "/"
			}
			names = append(names, name)
		}
	}
	return names, err
}

// localRedirect gives a Moved Permanently response.
// It does not convert relative paths to absolute paths like Redirect does.
func localRedirect(w http.ResponseWriter, r *http.Request, newPath string) {
	if q := r.URL.RawQuery; q != "" {
		newPath += "?" + q
	}
	w.Header().Set("Location", newPath)
	w.WriteHeader(http.StatusMovedPermanently)
}

// Sadly, http.dirList is not exported. Also it doesn't sort the list by
// default but we don't care about performance.
func dirList(w http.ResponseWriter, items []string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "<html><body><pre>")
	sort.Strings(items)
	for _, name := range items {
		name = html.EscapeString(name)
		fmt.Fprintf(w, "<a href=\"%s\">%s</a>\n", name, name)
	}
	io.WriteString(w, "</pre></body></html>")
}

type nodeCache struct {
	Node
	lastAccess time.Time
}

type entryCache struct {
	EntryFileSystem
	lastAccess time.Time
}

// Serves the NodesName directory and its virtual directory.
type nodeFileSystem struct {
	nodesDir string
	cas      CasTable
	maxItems int

	// Mutables
	mutex         sync.Mutex
	recentNodes   map[string]*nodeCache
	recentEntries map[string]*entryCache
}

func makeNodeFileSystem(nodesDir string, cas CasTable) http.Handler {
	return &nodeFileSystem{
		nodesDir:      nodesDir,
		cas:           cas,
		maxItems:      10,
		recentNodes:   map[string]*nodeCache{},
		recentEntries: map[string]*entryCache{},
	}
}

// Loads a node from the file system if found.
func (n *nodeFileSystem) getNode(url string) (*Node, string, error) {
	prefix := ""
	rest := url
	for rest != "" {
		i := strings.Index(rest, "/")
		if i == -1 {
			prefix += rest
			rest = ""
		} else {
			prefix += rest[:i+1]
			rest = rest[i+1:]
		}
		// Convert to OS file path.
		relPath := strings.Replace(strings.Trim(prefix, "/"), "/", string(filepath.Separator), 0)
		f, err := os.Open(path.Join(n.nodesDir, relPath))
		if err != nil {
			return nil, "", err
		}
		defer f.Close()
		stat, err := f.Stat()
		if err != nil {
			return nil, "", err
		}
		if !stat.IsDir() {
			node := &nodeCache{}
			if err := loadReaderAsJson(f, &node.Node); err == nil {
				node.lastAccess = time.Now()
				go n.updateNodeCache(prefix, node)
				return &node.Node, rest, err
			} else {
				return nil, "", err
			}
		}
	}
	// No error, didn't find anything.
	return nil, url, nil
}

// Tries to find the node in the cache by testing all the cached nodes. It's
// faster than touching the file system.
func (n *nodeFileSystem) findCachedNode(url string) (*Node, string) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	for key, node := range n.recentNodes {
		if strings.HasPrefix(url, key) {
			rest := url[len(key):]
			node.lastAccess = time.Now()
			return &node.Node, rest
		}
	}
	return nil, ""
}

func (n *nodeFileSystem) updateNodeCache(nodeName string, nodeObj *nodeCache) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.recentNodes[nodeName] = nodeObj
	for len(n.recentNodes) > n.maxItems {
		// Find the oldest and remove it.
		olderName := nodeName
		olderStamp := nodeObj.lastAccess
		for n, o := range n.recentNodes {
			if o.lastAccess.Before(olderStamp) {
				olderStamp = o.lastAccess
				olderName = n
			}
		}
		delete(n.recentNodes, olderName)
	}
}

func (n *nodeFileSystem) getEntry(entryName string) (*entryCache, error) {
	n.mutex.Lock()
	if entryObj, ok := n.recentEntries[entryName]; ok {
		entryObj.lastAccess = time.Now()
		n.mutex.Unlock()
		return entryObj, nil
	}
	n.mutex.Unlock()

	// Create a new entry without the lock.
	entryObj := &entryCache{EntryFileSystem: EntryFileSystem{cas: n.cas}}
	f, err := n.cas.Open(entryName)
	if err != nil {
		return nil, fmt.Errorf("Failed to load the entry file: %s", err)
	}
	defer f.Close()
	if err := loadReaderAsJson(f, &entryObj.entry); err != nil {
		return nil, err
	}
	go n.updateEntryCache(entryName, entryObj)
	return entryObj, nil
}

func (n *nodeFileSystem) updateEntryCache(entryName string, entryObj *entryCache) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.recentEntries[entryName] = entryObj
	for len(n.recentEntries) > n.maxItems {
		// Find the oldest and remove it.
		olderName := entryName
		olderStamp := entryObj.lastAccess
		for n, o := range n.recentEntries {
			if o.lastAccess.Before(olderStamp) {
				olderStamp = o.lastAccess
				olderName = n
			}
		}
		delete(n.recentEntries, olderName)
	}
}

func (n *nodeFileSystem) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "" || r.URL.Path[0] != '/' {
		http.Error(w, "Internal failure. nodeFileSystem received an invalid url: "+r.URL.Path, http.StatusNotImplemented)
		return
	}

	// Enumerate the file system to find back the node.
	name := r.URL.Path[1:]
	node, rest := n.findCachedNode(name)
	if node != nil {
		// Check manually for the root.
		if rest == "" && name[len(name)-1] != '/' {
			localRedirect(w, r, path.Base(r.URL.Path)+"/")
			return
		}
		// Fast path to entry virtual file system.
		r.URL.Path = "/" + rest
		n.serveObj(w, r, node)
		return
	}

	// The node isn't cached. Look at the file system.
	node, rest, err := n.getNode(name)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failure: %s", err), http.StatusNotFound)
		return
	}
	if node != nil {
		// Check manually for the root.
		if rest == "" && name[len(name)-1] != '/' {
			localRedirect(w, r, path.Base(r.URL.Path)+"/")
			return
		}
		// Redirect to entry virtual file system.
		r.URL.Path = "/" + rest
		n.serveObj(w, r, node)
		return
	}

	// It's actually browsing the nodes themselves. Read the directory entry if possible.
	if name != "" && name[len(name)-1] != '/' {
		localRedirect(w, r, path.Base(r.URL.Path)+"/")
		return
	}
	files, _ := readDirFancy(path.Join(n.nodesDir, name))
	dirList(w, files)
	return
}

// Either failed to load a Node or an Entry.
func (n *nodeFileSystem) Corruption(w http.ResponseWriter, format string, a ...interface{}) {
	n.cas.NeedFsck()
	str := fmt.Sprintf(format, a)
	http.Error(w, "Internal failure: "+str, http.StatusNotImplemented)
}

// Converts the Node request to a EntryFileSystem request. This loads the entry
// file and redirects to its virtual file system.
func (n *nodeFileSystem) serveObj(w http.ResponseWriter, r *http.Request, node *Node) {
	entryFs, err := n.getEntry(node.Entry)
	if err != nil {
		n.Corruption(w, "Failed to load Entry %s: %s", node.Entry, err)
		return
	}
	entryFs.ServeHTTP(w, r)
}

var cmdWeb = &Command{
	Run:       runWeb,
	UsageLine: "web",
	ShortDesc: "starts a web service to access the dumbcas",
	LongDesc:  "Serves each node as a full virtual tree of the archived files.",
	Flag:      GetCommonFlags(),
}

func webMain(port int, ready chan<- net.Listener, l *log.Logger) error {
	cas, err := CommonFlag(false, true)
	if err != nil {
		return err
	}

	nodesDir := path.Join(Root, NodesName)
	if !isDir(nodesDir) {
		return fmt.Errorf("Please archive something first into %s", Root)
	}

	serveMux := http.NewServeMux()

	x := http.StripPrefix("/content/retrieve/default", cas)
	serveMux.Handle("/content/retrieve/default/", Restrict(x, "GET"))
	x = http.StripPrefix("/content/retrieve/nodes", makeNodeFileSystem(nodesDir, cas))
	serveMux.Handle("/content/retrieve/nodes/", Restrict(x, "GET"))
	serveMux.Handle("/", Restrict(http.RedirectHandler("/content/retrieve/nodes/", http.StatusFound), "GET"))

	s := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: &LoggingHandler{serveMux, l},
	}
	ls, e := net.Listen("tcp", s.Addr)
	if e != nil {
		return e
	}

	_, portStr, _ := net.SplitHostPort(ls.Addr().String())
	l.Printf("Serving %s on port %s", Root, portStr)

	if ready != nil {
		ready <- ls
	}
	return s.Serve(ls)
}

// Flags.
var webPort int

func init() {
	cmdWeb.Flag.IntVar(&webPort, "port", 8010, "port number")
}

func runWeb(a *Application, cmd *Command, args []string) int {
	if len(args) != 0 {
		fmt.Fprintf(a.Err, "%s: Unsupported arguments.\n", a.Name)
		return 1
	}
	if err := webMain(webPort, nil, a.Log); err != nil {
		fmt.Fprintf(a.Err, "%s: %s\n", a.Name, err)
		return 1
	}
	// This is never executed.
	return 0
}
