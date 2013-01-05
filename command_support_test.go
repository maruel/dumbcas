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
	//"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

// Logging is a global object so it can't be checked for when tests are run in parallel.
var bufLog bytes.Buffer

var enableOutput = false

func init() {
	// Reduces output. Comment out to get more logs.
	if !enableOutput {
		log.SetOutput(&bufLog)
	}
	log.SetFlags(log.Lmicroseconds)
}

type TB struct {
	*testing.T
	bufLog bytes.Buffer
	bufOut bytes.Buffer
	bufErr bytes.Buffer
	log    *log.Logger
}

func MakeTB(t *testing.T) *TB {
	tb := &TB{T: t}
	tb.log = log.New(&tb.bufLog, "", log.Lmicroseconds)
	if enableOutput {
		tb.Verbose()
	}
	return tb
}

func PrintIf(b []byte, name string) {
	s := strings.TrimSpace(string(b))
	if len(s) != 0 {
		fmt.Fprintf(os.Stderr, "\n\\/ \\/ %s \\/ \\/\n%s\n/\\ /\\ %s /\\ /\\\n", name, s, name)
	}
}

// Reduces the amount of data in a stack trace.
// Trims the first 2 lines and remove the file paths and function pointers to
// only keep the file names and line numbers.
func ReduceStackTrace(b []byte) []byte {
	lines := strings.Split(string(b), "\n")
	if len(lines) > 2 {
		lines = lines[2:]
	}
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "\t") {
			// /path/to/file.go:<lineno> (<addr>)
			// TODO(maruel): Check on Windows.
			start := strings.LastIndex(lines[i], string(filepath.Separator))
			end := strings.LastIndex(lines[i], " ")
			if start != -1 && end != -1 {
				lines[i] = lines[i][start+1 : end]
			}
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// Prints the stack trace to ease debugging.
// It's slightly slower than an explicit condition in the test but its more compact.
func (t *TB) Assertf(truth bool, format string, values ...interface{}) {
	if !truth {
		PrintIf(t.bufOut.Bytes(), "STDOUT")
		PrintIf(t.bufErr.Bytes(), "STDERR")
		PrintIf(t.bufLog.Bytes(), "LOG")
		os.Stderr.Write([]byte("\n"))
		os.Stderr.Write(ReduceStackTrace(debug.Stack()))
		t.Fatalf(format, values...)
	}
}

func (t *TB) CheckBuffer(out, err bool) {
	if out {
		// Print Stderr to see what happened.
		t.Assertf(t.bufOut.Len() != 0, "Expected stdout")
	} else {
		t.Assertf(t.bufOut.Len() == 0, "Unexpected stdout")
	}

	if err {
		t.Assertf(t.bufErr.Len() != 0, "Expected stderr")
	} else {
		t.Assertf(t.bufErr.Len() == 0, "Unexpected stderr")
	}
	t.bufOut.Reset()
	t.bufErr.Reset()
}

func (tb *TB) Verbose() {
	if tb.bufLog.Len() != 0 {
		os.Stderr.Write(tb.bufLog.Bytes())
	}
	tb.log = log.New(os.Stderr, "", log.Lmicroseconds)
}

type ApplicationMock struct {
	*DefaultApplication
	*TB
}

func (a *ApplicationMock) GetOut() io.Writer {
	return &a.bufOut
}

func (a *ApplicationMock) GetErr() io.Writer {
	return &a.bufErr
}

func MakeAppMock(t *testing.T, a *DefaultApplication) *ApplicationMock {
	return &ApplicationMock{a, MakeTB(t)}
}

func TestHelp(t *testing.T) {
	t.Parallel()
	app := &DefaultApplication{
		Name:  "name",
		Title: "doc",
		Commands: []*Command{
			cmdHelp,
		},
	}
	a := MakeAppMock(t, app)
	args := []string{"help"}
	r := Run(a, args)
	a.Assertf(r == 0, "Unexpected return code %d", r)
	a.CheckBuffer(true, false)
}

func TestHelpBadFlag(t *testing.T) {
	t.Parallel()
	app := &DefaultApplication{
		Name:  "name",
		Title: "doc",
		Commands: []*Command{
			cmdHelp,
		},
	}
	a := MakeAppMock(t, app)
	args := []string{"help", "-foo"}
	r := Run(a, args)
	a.Assertf(r == 2, "Unexpected return code %d", r)
	a.CheckBuffer(false, true)
}

func TestHelpBadCommand(t *testing.T) {
	t.Parallel()
	app := &DefaultApplication{
		Name:  "name",
		Title: "doc",
		Commands: []*Command{
			cmdHelp,
		},
	}
	a := MakeAppMock(t, app)
	args := []string{"help", "non_existing_command"}
	r := Run(a, args)
	a.Assertf(r == 2, "Unexpected return code %d", r)
	a.CheckBuffer(false, true)
}

func TestBadCommand(t *testing.T) {
	t.Parallel()
	app := &DefaultApplication{
		Name:  "name",
		Title: "doc",
		Commands: []*Command{
			cmdHelp,
		},
	}
	a := MakeAppMock(t, app)
	args := []string{"non_existing_command"}
	r := Run(a, args)
	a.Assertf(r == 2, "Unexpected return code %d", r)
	a.CheckBuffer(false, true)
}

func TestReduceStackTrace(t *testing.T) {
	t.Parallel()
	tb := MakeTB(t)
	data := "/home/joe/gocode/src/github.com/maruel/dumbcas/command_support_test.go:93 (0x43acb9)\n" +
		"\tcom/maruel/dumbcas.(*TB).Assertf: os.Stderr.Write(ReduceStackTrace(debug.Stack()))\n" +
		"/home/joe/gocode/src/github.com/maruel/dumbcas/command_support_test.go:109 (0x43aeaf)\n" +
		"\tcom/maruel/dumbcas.(*TB).CheckBuffer: t.Assertf(t.bufErr.Len() != 0, \"Unexpected stderr\")\n" +
		"/home/joe/gocode/src/github.com/maruel/dumbcas/web_test.go:57 (0x440109)\n" +
		"\tcom/maruel/dumbcas.(*WebDumbcasAppMock).closeWeb: f.CheckBuffer(false, false)\n" +
		"/home/joe/gocode/src/github.com/maruel/dumbcas/web_test.go:147 (0x441a54)\n" +
		"\tcom/maruel/dumbcas.TestWeb: f.closeWeb()\n"

	// Much nicer!
	expected := "command_support_test.go:109\n" +
		"\tcom/maruel/dumbcas.(*TB).CheckBuffer: t.Assertf(t.bufErr.Len() != 0, \"Unexpected stderr\")\n" +
		"web_test.go:57\n" +
		"\tcom/maruel/dumbcas.(*WebDumbcasAppMock).closeWeb: f.CheckBuffer(false, false)\n" +
		"web_test.go:147\n" +
		"\tcom/maruel/dumbcas.TestWeb: f.closeWeb()\n"

	actual := string(ReduceStackTrace([]byte(data)))
	tb.Assertf(expected == actual, "ReduceStackTrace() failed parsing.\nActual:\n%s\n\nExpected:\n%s", expected, actual)
}
