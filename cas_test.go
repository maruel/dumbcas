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
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
)

type mockCasTable struct {
	entries  map[string][]byte
	needFsck bool
	*testing.T
}

func (a *ApplicationMock) MakeCasTable(rootDir string) (CasTable, error) {
	return makeCasTable(rootDir)
	/*
		if a.cas == nil {
			a.cas = &mockCasTable{make(map[string][]byte), false, a.T}
		}
		return a.cas, nil
	*/
}

func (m *mockCasTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write(m.entries[r.URL.Path[1:]])
}

func (m *mockCasTable) Enumerate() <-chan CasEntry {
	// First make a copy of the keys.
	keys := make([]string, len(m.entries))
	i := 0
	for k, _ := range m.entries {
		keys[i] = k
		i++
	}
	c := make(chan CasEntry)
	go func() {
		for _, k := range keys {
			c <- CasEntry{Item: k}
		}
		close(c)
	}()
	return c
}

func (m *mockCasTable) AddEntry(source io.Reader, hash string) error {
	data, err := ioutil.ReadAll(source)
	if err == nil {
		m.entries[hash] = data
	}
	return err
}

func (m *mockCasTable) Open(hash string) (ReadSeekCloser, error) {
	data, ok := m.entries[hash]
	if !ok {
		return nil, fmt.Errorf("Missing: %s", hash)
	}
	return Buffer{bytes.NewReader(data)}, nil
}

// Adds noop Close() to a bytes.Reader.
type Buffer struct {
	*bytes.Reader
}

func (b Buffer) Close() error {
	return nil
}

func (m *mockCasTable) Remove(item string) error {
	delete(m.entries, item)
	return nil
}

func (m *mockCasTable) NeedFsck() {
	m.needFsck = true
}

func (m *mockCasTable) WarnIfFsckIsNeeded() bool {
	return m.needFsck
}

func TestPrefixSpace(t *testing.T) {
	t.Parallel()
	type S struct {
		i int
		s string
	}
	checks := map[int]S{
		0: S{0, ""},
		1: S{16, "f"},
		2: S{256, "ff"},
		3: S{4096, "fff"},
		4: S{65536, "ffff"},
	}
	for prefixLength, s := range checks {
		x := prefixSpace(uint(prefixLength))
		if x != s.i {
			t.Fatalf("%d: %d != %d", prefixLength, x, s.i)
		}
		if x != 0 {
			res := fmt.Sprintf("%0*x", prefixLength, x-1)
			if res != s.s {
				t.Fatalf("%d: %s != %s", prefixLength, res, s.s)
			}
		}
	}
}
