/* Copyright 2012 Marc-Antoine Ruel. Licensed under the Apache License, Version
2.0 (the "License"); you may not use this file except in compliance with the
License.  You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0. Unless required by applicable law or
agreed to in writing, software distributed under the License is distributed on
an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
or implied. See the License for the specific language governing permissions and
limitations under the License. */

/* This package permits to implement subcommands support similar to what is
* supported by the 'go' tool.

TODO(maruel): Split off as a separate package if someone likes it. Please email
me if you'd like me to do it.
*/

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
)

// Application describes an application with subcommand support.
type Application interface {
	GetName() string
	GetTitle() string
	GetCommands() []*Command
	GetOut() io.Writer // Used for testing, should be normally os.Stdout.
	GetErr() io.Writer // Used for testing, should be normally os.Stderr.
}

// DefaultApplication implements all of Application interface's methods.
type DefaultApplication struct {
	Name     string
	Title    string
	Commands []*Command
}

func (a *DefaultApplication) GetName() string {
	return a.Name
}

func (a *DefaultApplication) GetTitle() string {
	return a.Title
}

func (a *DefaultApplication) GetCommands() []*Command {
	return a.Commands
}

func (a *DefaultApplication) GetOut() io.Writer {
	return os.Stdout
}

func (a *DefaultApplication) GetErr() io.Writer {
	return os.Stderr
}

// CommandRun is an initialized object representing a subcommand that is ready
// to be executed.
type CommandRun interface {
	Run(a Application, args []string) int
	GetFlags() *flag.FlagSet
}

// Command describes a subcommand. It has one generator to generate a command
// object which is executable. The purpose of this design is to enable safe
// parallel execution of test cases.
type Command struct {
	UsageLine  string
	ShortDesc  string
	LongDesc   string
	CommandRun func() CommandRun
}

// CommandRunBase implements GetFlags of CommandRun. It should be embedded in
// another struct that implements Run().
type CommandRunBase struct {
	Flags flag.FlagSet
}

func (c *CommandRunBase) GetFlags() *flag.FlagSet {
	return &c.Flags
}

// Name returns the command's name: the first word in the usage line.
func (c *Command) Name() string {
	name := c.UsageLine
	i := strings.Index(name, " ")
	if i >= 0 {
		name = name[:i]
	}
	return name
}

// usage prints out the general application usage.
func usage(out io.Writer, a Application) {
	usageTemplate := `{{.GetTitle}}

Usage:  {{.GetName}} [command] [arguments]

Commands:{{range .Commands}}
    {{.Name | printf "%-11s"}} {{.ShortDesc}}{{end}}

Use "{{.GetName}} help [command]" for more information about a command.

`
	tmpl(out, usageTemplate, a)
}

func getCommandUsageHandler(out io.Writer, a Application, c *Command, r CommandRun, helpUsed *bool) func() {
	return func() {
		helpTemplate := "{{.Cmd.LongDesc | trim | wrapWithLines}}usage:  {{.App.GetName}} {{.Cmd.UsageLine}}\n"
		dict := struct {
			App Application
			Cmd *Command
		}{a, c}
		tmpl(out, helpTemplate, dict)
		r.GetFlags().PrintDefaults()
		*helpUsed = true
	}
}

// Initializes the flags for a specific CommandRun.
func initCommand(a Application, c *Command, r CommandRun, out io.Writer, helpUsed *bool) {
	r.GetFlags().Usage = getCommandUsageHandler(out, a, c, r, helpUsed)
	r.GetFlags().SetOutput(out)
	r.GetFlags().Init(c.Name(), flag.ContinueOnError)
}

// Finds a Command by name and returns it if found.
func FindCommand(a Application, name string) *Command {
	for _, c := range a.GetCommands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

// Run runs the application, scheduling the subcommand.
func Run(a Application, args []string) int {
	var helpUsed bool

	// Process general flags first, mainly for -help.
	flag.Usage = func() {
		usage(a.GetErr(), a)
		helpUsed = true
	}

	// Do not parse during unit tests because flag.commandLine.errorHandling == ExitOnError. :(
	// It is safer to use a base class embedding CommandRunBase that is then
	// embedded by each CommandRun implementation to define flags available for
	// all commands.
	if args == nil {
		flag.Parse()
		args = flag.Args()
	}

	if len(args) < 1 {
		// Need a command.
		usage(a.GetErr(), a)
		return 2
	}

	if c := FindCommand(a, args[0]); c != nil {
		// Initialize the flags.
		r := c.CommandRun()
		initCommand(a, c, r, a.GetErr(), &helpUsed)
		r.GetFlags().Parse(args[1:])
		if helpUsed {
			return 0
		}
		return r.Run(a, r.GetFlags().Args())
	}

	fmt.Fprintf(a.GetErr(), "%s: unknown command %#q\n\nRun '%s help' for usage.\n", a.GetName(), args[0], a.GetName())
	return 2
}

// tmpl executes the given template text on data, writing the result to w.
func tmpl(w io.Writer, text string, data interface{}) {
	t := template.New("top")
	t.Funcs(template.FuncMap{"trim": strings.TrimSpace, "wrapWithLines": wrapWithLines})
	template.Must(t.Parse(text))
	if err := t.Execute(w, data); err != nil {
		panic(err)
	}
}

func wrapWithLines(s string) string {
	if s == "" {
		return s
	}
	return s + "\n\n"
}

var cmdHelp = &Command{
	UsageLine:  "help <command>",
	ShortDesc:  "prints help about a command",
	LongDesc:   "Prints an overview of every commands or information about a specific command.",
	CommandRun: func() CommandRun { return &helpRun{} },
}

type helpRun struct {
	CommandRunBase
}

func (c *helpRun) Run(a Application, args []string) int {
	if len(args) == 0 {
		usage(a.GetOut(), a)
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintf(a.GetErr(), "%s: Too many arguments given\n\nRun '%s help' for usage.\n", a.GetName(), a.GetName())
		return 2
	}
	// Redirects all output to Out.
	var helpUsed bool
	if cmd := FindCommand(a, args[0]); cmd != nil {
		// Initialize the flags.
		r := cmd.CommandRun()
		initCommand(a, cmd, r, a.GetErr(), &helpUsed)
		r.GetFlags().Usage()
		return 0
	}

	fmt.Fprintf(a.GetErr(), "%s: unknown command %#q\n\nRun '%s help' for usage.\n", a.GetName(), args[0], a.GetName())
	return 2
}
