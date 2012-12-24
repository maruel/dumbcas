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
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// The nodes are stored in a separate directory from the CAS store.
const nodesName = "nodes"

// Tags is a Nodes subdirectory, to implement the equivalent of permanent
// nodes. They are overwritten automatically.
const tagsName = "tags"

type Node struct {
	Entry   string
	Comment string `json:",omitempty"`
}

type NodesTable interface {
	http.Handler
	AddEntry(node *Node, name string) error
	// Temporary.
	Root() string
}

type nodesTable struct {
	nodesDir string
	cas      CasTable
	maxItems int
	hostname string
	l        *log.Logger

	mutex         sync.Mutex
	recentNodes   map[string]*nodeCache
	recentEntries map[string]*entryCache
}

type nodeCache struct {
	Node
	lastAccess time.Time
}

type entryCache struct {
	EntryFileSystem
	lastAccess time.Time
}

func LoadNodesTable(rootDir string, cas CasTable, l *log.Logger) (NodesTable, error) {
	nodesDir := path.Join(rootDir, nodesName)
	if err := os.Mkdir(nodesDir, 0750); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("LoadNodesTable(%s): Failed to create %s: %s\n", rootDir, nodesDir, err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("Failed to get the hostname: %s", err)
	}
	parts := strings.SplitN(hostname, ".", 2)
	hostname = parts[0]
	return &nodesTable{
		nodesDir:      nodesDir,
		cas:           cas,
		maxItems:      10,
		hostname:      hostname,
		l:             l,
		recentNodes:   map[string]*nodeCache{},
		recentEntries: map[string]*entryCache{},
	}, nil
}

func (n *nodesTable) Root() string {
	return n.nodesDir
}

func (n *nodesTable) AddEntry(node *Node, name string) error {
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("Failed to marshall internal state: %s", err)
	}
	now := time.Now().UTC()
	// Create one directory store per month.
	monthName := now.Format("2006-01")
	monthDir := path.Join(n.nodesDir, monthName)
	if err := os.MkdirAll(monthDir, 0750); err != nil && !os.IsExist(err) {
		return fmt.Errorf("Failed to create %s: %s\n", monthDir, err)
	}
	suffix := 0
	nodePath := ""
	for {
		nodeName := n.hostname + "_" + now.Format("2006-01-02_15-04-05") + "_" + name
		if suffix != 0 {
			nodeName += fmt.Sprintf("(%d)", suffix)
		}
		nodePath = path.Join(monthDir, nodeName)
		f, err := os.OpenFile(nodePath, os.O_WRONLY|os.O_EXCL|os.O_CREATE, 0640)
		if err != nil {
			// Try ad nauseam.
			suffix += 1
		} else {
			if _, err = f.Write(data); err != nil {
				return fmt.Errorf("Failed to write %s: %s", f.Name(), err)
			}
			n.l.Printf("Saved node: %s", path.Join(monthName, nodeName))
			break
		}
	}

	// Also update the tag by creating a symlink.
	tagsDir := path.Join(n.nodesDir, tagsName)
	if err := os.MkdirAll(tagsDir, 0750); err != nil && !os.IsExist(err) {
		return fmt.Errorf("Failed to create %s: %s\n", tagsDir, err)
	}
	tagPath := path.Join(tagsDir, name)
	relPath, err := filepath.Rel(tagsDir, nodePath)
	if err != nil {
		return err
	}
	// Ignore error.
	os.Remove(tagPath)
	if err := os.Symlink(relPath, tagPath); err != nil {
		return fmt.Errorf("Failed to create tag %s: %s", tagPath, err)
	}
	return nil
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

// Loads a node from the file system if found.
func (n *nodesTable) getNode(url string) (*Node, string, error) {
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
func (n *nodesTable) findCachedNode(url string) (*Node, string) {
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

func (n *nodesTable) updateNodeCache(nodeName string, nodeObj *nodeCache) {
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

func (n *nodesTable) getEntry(entryName string) (*entryCache, error) {
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

func (n *nodesTable) updateEntryCache(entryName string, entryObj *entryCache) {
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

// Serves the NodesName directory and its virtual directory.
func (n *nodesTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "" || r.URL.Path[0] != '/' {
		http.Error(w, "Internal failure. nodesTable received an invalid url: "+r.URL.Path, http.StatusNotImplemented)
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
func (n *nodesTable) corruption(w http.ResponseWriter, format string, a ...interface{}) {
	n.cas.NeedFsck()
	str := fmt.Sprintf(format, a)
	http.Error(w, "Internal failure: "+str, http.StatusNotImplemented)
}

// Converts the Node request to a EntryFileSystem request. This loads the entry
// file and redirects to its virtual file system.
func (n *nodesTable) serveObj(w http.ResponseWriter, r *http.Request, node *Node) {
	entryFs, err := n.getEntry(node.Entry)
	if err != nil {
		n.corruption(w, "Failed to load Entry %s: %s", node.Entry, err)
		return
	}
	entryFs.ServeHTTP(w, r)
}
