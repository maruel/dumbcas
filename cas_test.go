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

	"github.com/maruel/subcommands/subcommandstest"
	"github.com/maruel/ut"
)

// A working CasTable implementation that keeps all the data in memory.
type fakeCasTable struct {
	entries  map[string][]byte
	needFsck bool
	t        *subcommandstest.TB
}

func (a *DumbcasAppMock) MakeCasTable(rootDir string) (CasTable, error) {
	if a.cas == nil {
		a.cas = &fakeCasTable{make(map[string][]byte), false, a.TB}
	}
	return a.cas, nil
}

func (m *fakeCasTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.t.GetLog().Printf("fakeCasTable.ServeHTTP(%s)", r.URL.Path)
	w.Write(m.entries[r.URL.Path[1:]])
}

func (m *fakeCasTable) Enumerate() <-chan EnumerationEntry {
	// First make a copy of the keys.
	keys := make([]string, len(m.entries))
	i := 0
	for k := range m.entries {
		keys[i] = k
		i++
	}
	m.t.GetLog().Printf("fakeCasTable.Enumerate() %d", len(keys))
	c := make(chan EnumerationEntry)
	go func() {
		for _, k := range keys {
			c <- EnumerationEntry{Item: k}
		}
		close(c)
	}()
	return c
}

func (m *fakeCasTable) AddEntry(source io.Reader, item string) error {
	m.t.GetLog().Printf("fakeCasTable.AddEntry(%s)", item)
	if _, ok := m.entries[item]; ok {
		return os.ErrExist
	}
	data, err := ioutil.ReadAll(source)
	if err == nil {
		m.entries[item] = data
	}
	return err
}

func (m *fakeCasTable) Open(item string) (ReadSeekCloser, error) {
	m.t.GetLog().Printf("fakeCasTable.Open(%s)", item)
	data, ok := m.entries[item]
	if !ok {
		return nil, fmt.Errorf("Missing: %s", item)
	}
	return Buffer{bytes.NewReader(data)}, nil
}

func (m *fakeCasTable) Remove(item string) error {
	m.t.GetLog().Printf("fakeCasTable.Remove(%s)", item)
	if _, ok := m.entries[item]; !ok {
		return os.ErrNotExist
	}
	delete(m.entries, item)
	return nil
}

func (m *fakeCasTable) SetFsckBit() {
	m.t.GetLog().Printf("fakeCasTable.SetFsckBit()")
	m.needFsck = true
}

func (m *fakeCasTable) GetFsckBit() bool {
	m.t.GetLog().Printf("fakeCasTable.GetFsckBit() %t", m.needFsck)
	return m.needFsck
}

func (m *fakeCasTable) ClearFsckBit() {
	m.t.GetLog().Printf("fakeCasTable.ClearFsckBit()")
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
func EnumerateCasAsList(t *subcommandstest.TB, cas CasTable) []string {
	items := []string{}
	for v := range cas.Enumerate() {
		ut.AssertEqual(t, nil, v.Error)
		// Hardcoded for sha1.
		ut.AssertEqual(t, 40, len(v.Item))
		items = append(items, v.Item)
	}
	sort.Strings(items)
	return items
}

func TestFakeCasTable(t *testing.T) {
	t.Parallel()
	tb := subcommandstest.MakeTB(t)
	cas := &fakeCasTable{make(map[string][]byte), false, tb}
	testCasTableImpl(tb, cas)
}

func testCasTableImpl(t *subcommandstest.TB, cas CasTable) {
	items := EnumerateCasAsList(t, cas)
	ut.AssertEqual(t, 0, len(items))

	file1, err := AddBytes(cas, []byte("content1"))
	ut.AssertEqual(t, nil, err)

	items = EnumerateCasAsList(t, cas)
	ut.AssertEqual(t, []string{file1}, items)

	// Add the same content.
	file2, err := AddBytes(cas, []byte("content1"))
	ut.AssertEqualf(t, true, os.IsExist(err), "Unexpected error: %s", err)
	ut.AssertEqual(t, file1, file2)

	items = EnumerateCasAsList(t, cas)
	ut.AssertEqual(t, []string{file1}, items)

	f, err := cas.Open(file1)
	ut.AssertEqual(t, nil, err)

	data, err := ioutil.ReadAll(f)
	f.Close()
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, "content1", string(data))

	_, err = cas.Open("0")
	ut.AssertEqual(t, false, err == nil)

	err = cas.Remove(file1)
	ut.AssertEqual(t, nil, err)

	err = cas.Remove(file1)
	ut.AssertEqual(t, false, err == nil)

	// Test fsck bit.
	ut.AssertEqual(t, false, cas.GetFsckBit())
	cas.SetFsckBit()
	ut.AssertEqual(t, true, cas.GetFsckBit())
	cas.ClearFsckBit()
	ut.AssertEqual(t, false, cas.GetFsckBit())
}
