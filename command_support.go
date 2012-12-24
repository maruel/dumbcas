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
	"log"
	"os"
	"strings"
	"text/template"
)

type Application struct {
	Name     string
	Title    string
	Commands []*Command
	Out      io.Writer
	Err      io.Writer
	Log      *log.Logger
}

type Command struct {
	Run       func(a *Application, cmd *Command, args []string) int
	UsageLine string
	ShortDesc string
	LongDesc  string
	Flag      flag.FlagSet
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

var usageTemplate = `{{.Title}}

Usage:  {{.Name}} [command] [arguments]

Commands:{{range .Commands}}
    {{.Name | printf "%-11s"}} {{.ShortDesc}}{{end}}

Use "{{.Name}} help [command]" for more information about a command.

`

func (a *Application) Usage() {
	tmpl(a.Err, usageTemplate, a)
}

func getCommandUsageHandler(a *Application, cmd *Command) func() {
	return func() {
		helpTemplate := "{{.Cmd.LongDesc | trim | wrapWithLines}}usage:  {{.App.Name}} {{.Cmd.UsageLine}}\n"
		dict := struct {
			App *Application
			Cmd *Command
		}{a, cmd}
		tmpl(a.Err, helpTemplate, dict)
		cmd.Flag.PrintDefaults()
	}
}

// Runs the application, scheduling the subcommand.
func (a *Application) Run(args []string) int {
	if a.Out == nil {
		a.Out = os.Stdout
	}
	if a.Err == nil {
		a.Err = os.Stderr
	}
	if a.Log == nil {
		a.Log = log.New(a.Err, "", log.LstdFlags)
	}
	// Initialize commands.
	for _, cmd := range a.Commands {
		cmd.Flag.Usage = getCommandUsageHandler(a, cmd)
		cmd.Flag.SetOutput(a.Err)
		cmd.Flag.Init(cmd.Name(), flag.ContinueOnError)
	}

	// Process general flags first, mainly for -help.
	flag.Usage = func() {
		a.Usage()
	}

	// Defaults; do not parse during unit tests because flag.commandLine.errorHandling == ExitOnError. :(
	if args == nil {
		flag.Parse()
		args = flag.Args()
	}

	if len(args) < 1 {
		// Need a command.
		a.Usage()
		return 2
	}

	for _, cmd := range a.Commands {
		if cmd.Name() == args[0] {
			cmd.Flag.Parse(args[1:])
			return cmd.Run(a, cmd, cmd.Flag.Args())
		}
	}

	fmt.Fprintf(a.Err, "%s: unknown command %#q\n\nRun '%s help' for usage.\n", a.Name, args[0], a.Name)
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
	Run:       runHelp,
	UsageLine: "help <command>",
	ShortDesc: "prints help about a command",
	LongDesc:  "Prints an overview of every commands or information about a specific command.",
}

func runHelp(a *Application, cmd *Command, args []string) int {
	// Redirect all output to Out.
	a.Err = a.Out
	if len(args) == 0 {
		a.Usage()
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintf(a.Err, "%s: Too many arguments given\n\nRun '%s help' for usage.\n", a.Name, a.Name)
		return 2
	}

	for _, cmdFound := range a.Commands {
		if cmdFound.Name() == args[0] {
			cmdFound.Flag.Usage()
			return 0
		}
	}

	fmt.Fprintf(a.Err, "%s: unknown command %#q\n\nRun '%s help' for usage.\n", a.Name, args[0], a.Name)
	return 2
}
