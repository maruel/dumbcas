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
	"flag"
	"io"
	"testing"
)

type ApplicationMock struct {
	DefaultApplication
	*testing.T
	bufOut bytes.Buffer
	bufErr bytes.Buffer
}

func (a *ApplicationMock) GetOut() io.Writer {
	return &a.bufOut
}

func (a *ApplicationMock) GetErr() io.Writer {
	return &a.bufErr
}

func (a *ApplicationMock) checkBuffer(out, err bool) {
	if out {
		if a.bufOut.Len() == 0 {
			// Print Stderr to see what happened.
			a.Fatalf("Expected buffer:\n%s", a.bufErr.String())
		}
	} else {
		if a.bufOut.Len() != 0 {
			a.Fatalf("Unexpected buffer:\n%s", a.bufOut.String())
		}
	}

	if err {
		if a.bufErr.Len() == 0 {
			a.Fatalf("Expected buffer:\n%s", a.bufOut.String())
		}
	} else {
		if a.bufErr.Len() != 0 {
			a.Fatalf("Unexpected buffer:\n%s", a.bufErr.String())
		}
	}
	a.bufOut.Reset()
	a.bufErr.Reset()
}

type CommandMock struct {
	Command
	flags flag.FlagSet
}

func (c *CommandMock) GetFlags() *flag.FlagSet {
	return &c.flags
}

func makeAppMock(t *testing.T) *ApplicationMock {
	a := &ApplicationMock{
		DefaultApplication: application,
		testing.T:          t,
	}
	for i, c := range a.Commands {
		a.Commands[i] = &CommandMock{c, *c.GetFlags()}
	}
	return a
}

func TestHelp(t *testing.T) {
	t.Parallel()
	a := makeAppMock(t)
	args := []string{"help"}
	if returncode := Run(a, args); returncode != 0 {
		a.Fatal("Unexpected return code", returncode)
	}
	a.checkBuffer(true, false)
}

func TestHelpBadFlag(t *testing.T) {
	t.Parallel()
	a := makeAppMock(t)
	args := []string{"help", "-foo"}
	// TODO(maruel): This is inconsistent.
	if returncode := Run(a, args); returncode != 0 {
		a.Fatal("Unexpected return code", returncode)
	}
	a.checkBuffer(false, true)
}

func TestHelpBadCommand(t *testing.T) {
	t.Parallel()
	a := makeAppMock(t)
	args := []string{"help", "non_existing_command"}
	if returncode := Run(a, args); returncode != 2 {
		a.Fatal("Unexpected return code", returncode)
	}
	a.checkBuffer(false, true)
}

func TestBadCommand(t *testing.T) {
	t.Parallel()
	a := makeAppMock(t)
	args := []string{"non_existing_command"}
	if returncode := Run(a, args); returncode != 2 {
		a.Fatal("Unexpected return code", returncode)
	}
	a.checkBuffer(false, true)
}
