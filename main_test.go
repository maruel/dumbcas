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
	"testing"

	"github.com/maruel/subcommands"
	"github.com/maruel/subcommands/subcommandstest"
	"github.com/maruel/ut"
)

func init() {
	subcommandstest.DisableLogOutput()
}

// Creates a copy of the application so it can be tested concurrently.
type DumbcasAppMock struct {
	*subcommandstest.ApplicationMock
	// Statefullness
	cache *fakeCache
	cas   CasTable
	nodes NodesTable
}

func (a *DumbcasAppMock) Run(args []string, expected int) {
	a.GetLog().Printf("%s", args)
	returncode := subcommands.Run(a, args)
	ut.AssertEqual(a, expected, returncode)
}

func makeDumbcasAppMock(t *testing.T) *DumbcasAppMock {
	return &DumbcasAppMock{ApplicationMock: subcommandstest.MakeAppMock(t, application)}
}

func TestMainHelp(t *testing.T) {
	t.Parallel()
	a := subcommandstest.MakeAppMock(t, application)
	args := []string{"help"}
	r := subcommands.Run(a, args)
	ut.AssertEqual(t, 0, r)
	a.CheckBuffer(true, false)
}
