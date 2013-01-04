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
	"log"
	"net/http"
	"os"
	"testing"
)

type mockCasTable struct {
	entries  map[string][]byte
	needFsck bool
	t        *testing.T
	log      *log.Logger
}

func (a *DumbcasAppMock) MakeCasTable(rootDir string) (CasTable, error) {
	//return makeCasTable(rootDir)
	if a.cas == nil {
		a.cas = &mockCasTable{make(map[string][]byte), false, a.T, a.log}
	}
	return a.cas, nil
}

func (m *mockCasTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.log.Printf("mockCasTable.ServeHTTP(%s)", r.URL.Path)
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
	m.log.Printf("mockCasTable.Enumerate() %d", len(keys))
	c := make(chan CasEntry)
	go func() {
		for _, k := range keys {
			c <- CasEntry{Item: k}
		}
		close(c)
	}()
	return c
}

func (m *mockCasTable) AddEntry(source io.Reader, item string) error {
	m.log.Printf("mockCasTable.AddEntry(%s)", item)
	if _, ok := m.entries[item]; ok {
		return os.ErrExist
	}
	data, err := ioutil.ReadAll(source)
	if err == nil {
		m.entries[item] = data
	}
	return err
}

func (m *mockCasTable) Open(item string) (ReadSeekCloser, error) {
	m.log.Printf("mockCasTable.Open(%s)", item)
	data, ok := m.entries[item]
	if !ok {
		return nil, fmt.Errorf("Missing: %s", item)
	}
	return Buffer{bytes.NewReader(data)}, nil
}

func (m *mockCasTable) Remove(item string) error {
	m.log.Printf("mockCasTable.Remove(%s)", item)
	if _, ok := m.entries[item]; !ok {
		return os.ErrNotExist
	}
	delete(m.entries, item)
	return nil
}

func (m *mockCasTable) SetFsckBit() {
	m.log.Printf("mockCasTable.SetFsckBit()")
	m.needFsck = true
}

func (m *mockCasTable) GetFsckBit() bool {
	m.log.Printf("mockCasTable.GetFsckBit() %t", m.needFsck)
	return m.needFsck
}

func (m *mockCasTable) ClearFsckBit() {
	m.log.Printf("mockCasTable.ClearFsckBit()")
	m.needFsck = false
}

// Adds noop Close() to a bytes.Reader.
type Buffer struct {
	*bytes.Reader
}

func (b Buffer) Close() error {
	return nil
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

func TestCasTableImpl(t *testing.T) {
	t.Parallel()
	tempData := makeTempDir(t, "cas")
	defer removeTempDir(tempData)

	cas, err := makeCasTable(tempData)
	if err != nil {
		t.Fatal(err)
	}
	testCasTableImpl(t, cas)
}

func TestCasTableNode(t *testing.T) {
	t.Parallel()

	log := getLog(false)
	cas := &mockCasTable{make(map[string][]byte), false, t, log}
	testCasTableImpl(t, cas)
}

func testCasTableImpl(t *testing.T, cas CasTable) {
	for _ = range cas.Enumerate() {
		t.Fatal("Found unexpected value")
	}

	file1, err := AddBytes(cas, []byte("content1"))
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for v := range cas.Enumerate() {
		if v.Item != file1 {
			t.Fatal("Found unexpected value")
		}
		count++
	}
	if count != 1 {
		t.Fatalf("Found %d items", count)
	}

	// Add the same content.
	file2, err := AddBytes(cas, []byte("content1"))
	if !os.IsExist(err) {
		t.Fatal(err)
	}
	if file1 != file2 {
		t.Fatal("Hash mismatch")
	}
	count = 0
	for v := range cas.Enumerate() {
		if v.Item != file1 {
			t.Fatal("Found unexpected value")
		}
		count++
	}
	if count != 1 {
		t.Fatalf("Found %d items", count)
	}

	f, err := cas.Open(file1)
	if err != nil {
		t.Fatal(err)
	}
	data, err := ioutil.ReadAll(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "content1" {
		t.Fatal(string(data))
	}
	_, err = cas.Open("0")
	if err == nil {
		t.Fatal("Unexpected success")
	}
	if err := cas.Remove(file1); err != nil {
		t.Fatal(err)
	}
	if err := cas.Remove(file1); err == nil {
		t.Fatal("Unexpected success")
	}

	// Test fsck bit.
	if cas.GetFsckBit() {
		t.Fatal("Unexpected")
	}
	cas.SetFsckBit()
	if !cas.GetFsckBit() {
		t.Fatal("Unexpected")
	}
	cas.ClearFsckBit()
	if cas.GetFsckBit() {
		t.Fatal("Unexpected")
	}

}
