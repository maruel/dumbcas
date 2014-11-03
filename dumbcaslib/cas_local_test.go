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
	"fmt"
	"testing"

	"github.com/maruel/ut"
)

func TestPrefixSpace(t *testing.T) {
	t.Parallel()
	type S struct {
		i int
		s string
	}
	checks := map[int]S{
		0: {0, ""},
		1: {16, "f"},
		2: {256, "ff"},
		3: {4096, "fff"},
		4: {65536, "ffff"},
	}
	for prefixLength, s := range checks {
		x := prefixSpace(uint(prefixLength))
		ut.AssertEqualIndex(t, prefixLength, x, s.i)
		if x != 0 {
			res := fmt.Sprintf("%0*x", prefixLength, x-1)
			ut.AssertEqualIndex(t, prefixLength, res, s.s)
		}
	}
}

func TestCasTableImpl(t *testing.T) {
	t.Parallel()
	tempData := makeTempDir(t, "cas")
	defer removeDir(t, tempData)

	cas, err := MakeLocalCasTable(tempData)
	ut.AssertEqual(t, nil, err)
	testCasTableImpl(t, cas)
}
