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
	"github.com/maruel/subcommands/subcommandstest"
	"testing"
)

func TestNodesTable(t *testing.T) {
	t.Parallel()
	tb := subcommandstest.MakeTB(t)
	tempData := makeTempDir(tb, "nodes")
	defer removeTempDir(tempData)

	// Explicitely use a mocked CasTable.
	cas := &mockCasTable{make(map[string][]byte), false, tb}
	nodes, err := loadLocalNodesTable(tempData, cas, tb.GetLog())
	tb.Assertf(err == nil, "Unexpected error: %s", err)

	testNodesTableImpl(tb, cas, nodes)
}
