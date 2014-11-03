/* Copyright 2012 Marc-Antoine Ruel. Licensed under the Apache License, Version
2.0 (the "License"); you may not use this file except in compliance with the
License.  You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0. Unless required by applicable law or
agreed to in writing, software distributed under the License is distributed on
an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
or implied. See the License for the specific language governing permissions and
limitations under the License. */

package dumbcaslib

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
)

// CasTable describes the interface to a content-addressed-storage.
type CasTable interface {
	Table
	// AddEntry adds a node to the table.
	AddEntry(source io.Reader, name string) error
	// SetFsckBit sets the bit that the table needs to be checked for consistency.
	SetFsckBit()
	// GetFsckBit returns if the fsck bit is set.
	GetFsckBit() bool
	// ClearFsckBit clears the fsck bit.
	ClearFsckBit()
}

// EnumerateCasAsList returns a sorted list of all the entries in a CasTable.
// It is meant to be used in test.
func EnumerateCasAsList(cas CasTable) ([]string, error) {
	items := []string{}
	for v := range cas.Enumerate() {
		if v.Error != nil {
			return nil, v.Error
		}
		items = append(items, v.Item)
	}
	sort.Strings(items)
	return items, nil
}

// MakeMemoryCasTable returns a CasTable implementation that keeps all the data
// in memory. Is it useful for testing.
func MakeMemoryCasTable() CasTable {
	return &memoryCasTable{make(map[string][]byte), false}
}

type memoryCasTable struct {
	entries  map[string][]byte
	needFsck bool
}

func (m *memoryCasTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write(m.entries[r.URL.Path[1:]])
}

func (m *memoryCasTable) Enumerate() <-chan EnumerationEntry {
	// First make a copy of the keys.
	keys := make([]string, len(m.entries))
	i := 0
	for k := range m.entries {
		keys[i] = k
		i++
	}
	c := make(chan EnumerationEntry)
	go func() {
		for _, k := range keys {
			c <- EnumerationEntry{Item: k}
		}
		close(c)
	}()
	return c
}

func (m *memoryCasTable) AddEntry(source io.Reader, item string) error {
	if _, ok := m.entries[item]; ok {
		return os.ErrExist
	}
	data, err := ioutil.ReadAll(source)
	if err == nil {
		m.entries[item] = data
	}
	return err
}

func (m *memoryCasTable) Open(item string) (ReadSeekCloser, error) {
	data, ok := m.entries[item]
	if !ok {
		return nil, fmt.Errorf("Missing: %s", item)
	}
	return closableBuffer{bytes.NewReader(data)}, nil
}

func (m *memoryCasTable) Remove(item string) error {
	if _, ok := m.entries[item]; !ok {
		return os.ErrNotExist
	}
	delete(m.entries, item)
	return nil
}

func (m *memoryCasTable) SetFsckBit() {
	m.needFsck = true
}

func (m *memoryCasTable) GetFsckBit() bool {
	return m.needFsck
}

func (m *memoryCasTable) ClearFsckBit() {
	m.needFsck = false
}

func (m *memoryCasTable) Corrupt() {
	m.entries[Sha1Bytes([]byte{0, 1})] = []byte("content5")
}
