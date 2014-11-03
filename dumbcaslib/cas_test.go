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
	"io/ioutil"
	"os"
	"testing"

	"github.com/maruel/ut"
)

func TestFakeCasTable(t *testing.T) {
	t.Parallel()
	cas := MakeMemoryCasTable()
	testCasTableImpl(t, cas)
}

func testCasTableImpl(t testing.TB, cas CasTable) {
	items, err := EnumerateCasAsList(cas)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, []string{}, items)

	file1, err := AddBytes(cas, []byte("content1"))
	ut.AssertEqual(t, nil, err)

	items, err = EnumerateCasAsList(cas)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, []string{file1}, items)

	// Add the same content.
	file2, err := AddBytes(cas, []byte("content1"))
	ut.AssertEqualf(t, true, os.IsExist(err), "Unexpected error: %s", err)
	ut.AssertEqual(t, file1, file2)

	items, err = EnumerateCasAsList(cas)
	ut.AssertEqual(t, nil, err)
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
