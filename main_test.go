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
	"log"
	"net"
	"os"
	"runtime/debug"
	"testing"
)

type DumbcasAppMock struct {
	ApplicationMock
	log *log.Logger
	// Statefullness
	cache *mockCache
	cas   CasTable
	nodes NodesTable
	// Optional stuff
	socket  net.Listener
	closed  chan bool
	baseUrl string
}

func (a *DumbcasAppMock) GetLog() *log.Logger {
	return a.log
}

// Prints the stack trace to ease debugging.
// It's slightly slower than an explicit condition in the test but its more compact.
func (d *DumbcasAppMock) Assertf(truth bool, fmt string, values ...interface{}) {
	if !truth {
		// Print the log back log first.
		// TODO: os.Stderr.Write(log.Buffer())
		os.Stderr.Write(debug.Stack())
		d.Fatalf(fmt, values...)
	}
}

func (a *DumbcasAppMock) Run(args []string, expected int) {
	a.GetLog().Printf("%s", args)
	returncode := Run(a, args)
	a.Assertf(returncode == expected, "Unexpected return code %d", returncode)
}

func makeDumbcasAppMock(t *testing.T, verbose bool) *DumbcasAppMock {
	a := &DumbcasAppMock{
		ApplicationMock: *makeAppMock(t),
		log:             getLog(verbose),
		closed:          make(chan bool),
	}
	return a
}
