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
	"fmt"
	"github.com/maruel/subcommands/subcommandstest"
	"testing"
)

func TestPrefixSpace(t *testing.T) {
	t.Parallel()
	tb := subcommandstest.MakeTB(t)
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
	tb := subcommandstest.MakeTB(t)
	tempData := makeTempDir(tb, "cas")
	defer removeTempDir(tempData)

	cas, err := makeLocalCasTable(tempData)
	tb.Assertf(err == nil, "Unexpected error: %s", err)
	testCasTableImpl(tb, cas)
}
