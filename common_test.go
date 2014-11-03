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
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maruel/dumbcas/dumbcaslib"
	"github.com/maruel/ut"
)

func makeTempDir(t testing.TB, name string) string {
	name, err := ioutil.TempDir("", "dumbcas_"+name)
	ut.AssertEqual(t, nil, err)
	return name
}

func removeDir(t testing.TB, tempDir string) {
	err := os.RemoveAll(tempDir)
	ut.AssertEqual(t, nil, err)
}

func createTree(rootDir string, tree map[string]string) error {
	for relPath, content := range tree {
		base := filepath.Dir(relPath)
		if base != "." {
			if err := os.MkdirAll(filepath.Join(rootDir, base), 0700); err != nil && !os.IsExist(err) {
				return err
			}
		}
		f, err := os.Create(filepath.Join(rootDir, relPath))
		if err != nil {
			return err
		}
		f.WriteString(content)
		f.Sync()
		f.Close()
	}
	return nil
}

// ToSet converts a string slice to a map of string acting as a set.
func ToSet(i []string) map[string]bool {
	out := make(map[string]bool)
	for _, v := range i {
		out[v] = true
	}
	return out
}

// Sub substracts list b from list a; e.g. it retuns "a-b".
// It keeps the a slice ordered.
func Sub(a []string, b []string) []string {
	bMap := ToSet(b)
	out := []string{}
	for _, v := range a {
		if !bMap[v] {
			out = append(out, v)
		}
	}
	return out
}

func sha1String(content string) string {
	hash := sha1.New()
	io.WriteString(hash, content)
	return hex.EncodeToString(hash.Sum(nil))
}

// marshalData returns the tree of sha1s and the json encoded Node as bytes.
func marshalData(t testing.TB, tree map[string]string) (map[string]string, []byte) {
	sha1tree := map[string]string{}
	entries := &dumbcaslib.Entry{}
	for k, v := range tree {
		h := dumbcaslib.Sha1Bytes([]byte(v))
		sha1tree[k] = h
		e := entries
		parts := strings.Split(k, "/")
		for i := 0; i < len(parts)-1; i++ {
			if e.Files == nil {
				e.Files = map[string]*dumbcaslib.Entry{}
			}
			if e.Files[parts[i]] == nil {
				e.Files[parts[i]] = &dumbcaslib.Entry{}
			}
			e = e.Files[parts[i]]
		}
		if e.Files == nil {
			e.Files = map[string]*dumbcaslib.Entry{}
		}
		e.Files[parts[len(parts)-1]] = &dumbcaslib.Entry{
			Sha1: h,
			Size: int64(len(v)),
		}
	}

	// Then process entries itself.
	data, err := json.Marshal(entries)
	ut.AssertEqual(t, nil, err)
	return sha1tree, data
}

// archiveData archives a tree fictious data.
// Returns (tree of sha1s, name of the node, sha1 of the node entry).
// Accept the paths as posix.
func archiveData(t testing.TB, cas dumbcaslib.CasTable, nodes dumbcaslib.NodesTable, tree map[string]string) (map[string]string, string, string) {
	sha1tree, entries := marshalData(t, tree)
	for k, v := range tree {
		err := cas.AddEntry(bytes.NewBuffer([]byte(v)), sha1tree[k])
		ut.AssertEqualf(t, true, err == nil || err == os.ErrExist, "Unexpected error: %s", err)
	}
	entrySha1, err := dumbcaslib.AddBytes(cas, entries)
	ut.AssertEqual(t, nil, err)

	// And finally add the node.
	now := time.Now().UTC()
	nodeName, err := nodes.AddEntry(&dumbcaslib.Node{entrySha1, "useful comment"}, "fictious")
	ut.AssertEqual(t, nil, err)
	ut.AssertEqualf(t, true, strings.HasPrefix(nodeName, now.Format("2006-01")+string(filepath.Separator)), "Invalid node name %s", nodeName)
	return sha1tree, nodeName, entrySha1
}
