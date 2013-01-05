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
	"os"
	"sort"
	"testing"
)

type mockCasTable struct {
	entries  map[string][]byte
	needFsck bool
	t        *TB
}

func (a *DumbcasAppMock) MakeCasTable(rootDir string) (CasTable, error) {
	//return makeCasTable(rootDir)
	if a.cas == nil {
		a.cas = &mockCasTable{make(map[string][]byte), false, a.TB}
	}
	return a.cas, nil
}

func (m *mockCasTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.t.log.Printf("mockCasTable.ServeHTTP(%s)", r.URL.Path)
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
	m.t.log.Printf("mockCasTable.Enumerate() %d", len(keys))
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
	m.t.log.Printf("mockCasTable.AddEntry(%s)", item)
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
	m.t.log.Printf("mockCasTable.Open(%s)", item)
	data, ok := m.entries[item]
	if !ok {
		return nil, fmt.Errorf("Missing: %s", item)
	}
	return Buffer{bytes.NewReader(data)}, nil
}

func (m *mockCasTable) Remove(item string) error {
	m.t.log.Printf("mockCasTable.Remove(%s)", item)
	if _, ok := m.entries[item]; !ok {
		return os.ErrNotExist
	}
	delete(m.entries, item)
	return nil
}

func (m *mockCasTable) SetFsckBit() {
	m.t.log.Printf("mockCasTable.SetFsckBit()")
	m.needFsck = true
}

func (m *mockCasTable) GetFsckBit() bool {
	m.t.log.Printf("mockCasTable.GetFsckBit() %t", m.needFsck)
	return m.needFsck
}

func (m *mockCasTable) ClearFsckBit() {
	m.t.log.Printf("mockCasTable.ClearFsckBit()")
	m.needFsck = false
}

// Adds noop Close() to a bytes.Reader.
type Buffer struct {
	*bytes.Reader
}

func (b Buffer) Close() error {
	return nil
}

// Returns a sorted list of all the entries.
func EnumerateCasAsList(t *TB, cas CasTable) []string {
	items := []string{}
	for v := range cas.Enumerate() {
		t.Assertf(v.Error == nil, "Unexpected failure")
		// Hardcoded for sha1.
		t.Assertf(len(v.Item) == 40, "Unexpected sha1 entry %s", v.Item)
		items = append(items, v.Item)
	}
	sort.Strings(items)
	return items
}

func Equals(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPrefixSpace(t *testing.T) {
	t.Parallel()
	tb := MakeTB(t)
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
		tb.Assertf(x == s.i, "%d: %d != %d", prefixLength, x, s.i)
		if x != 0 {
			res := fmt.Sprintf("%0*x", prefixLength, x-1)
			tb.Assertf(res == s.s, "%d: %s != %s", prefixLength, res, s.s)
		}
	}
}

func TestCasTableImpl(t *testing.T) {
	t.Parallel()
	tb := MakeTB(t)
	tempData := makeTempDir(tb, "cas")
	defer removeTempDir(tempData)

	cas, err := makeCasTable(tempData)
	tb.Assertf(err == nil, "Unexpected error: %s", err)
	testCasTableImpl(tb, cas)
}

func TestCasTableNode(t *testing.T) {
	t.Parallel()
	tb := MakeTB(t)
	cas := &mockCasTable{make(map[string][]byte), false, tb}
	testCasTableImpl(tb, cas)
}

func testCasTableImpl(t *TB, cas CasTable) {
	items := EnumerateCasAsList(t, cas)
	t.Assertf(len(items) == 0, "Found unexpected values: %q", items)

	file1, err := AddBytes(cas, []byte("content1"))
	t.Assertf(err == nil, "Unexpected error: %s", err)

	items = EnumerateCasAsList(t, cas)
	t.Assertf(Equals(items, []string{file1}), "Found unexpected values: %q != %s", items, file1)

	// Add the same content.
	file2, err := AddBytes(cas, []byte("content1"))
	t.Assertf(os.IsExist(err), "Unexpected error: %s", err)
	t.Assertf(file1 == file2, "Hash mismatch %s != %s", file1, file2)

	items = EnumerateCasAsList(t, cas)
	t.Assertf(Equals(items, []string{file1}), "Found unexpected values: %q != %s", items, file1)

	f, err := cas.Open(file1)
	t.Assertf(err == nil, "Unexpected error: %s", err)

	data, err := ioutil.ReadAll(f)
	f.Close()
	t.Assertf(err == nil, "Unexpected error: %s", err)
	t.Assertf(string(data) == "content1", "Unexpected value: %s", data)

	_, err = cas.Open("0")
	t.Assertf(err != nil, "Unexpected success")

	err = cas.Remove(file1)
	t.Assertf(err == nil, "Unexpected error: %s", err)

	err = cas.Remove(file1)
	t.Assertf(err != nil, "Unexpected success")

	// Test fsck bit.
	t.Assertf(!cas.GetFsckBit(), "Unexpected fsck bit is set")
	cas.SetFsckBit()
	t.Assertf(cas.GetFsckBit(), "Unexpected fsck bit is unset")
	cas.ClearFsckBit()
	t.Assertf(!cas.GetFsckBit(), "Unexpected fsck bit is set")
}
