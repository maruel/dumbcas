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

type Application interface {
	GetName() string
	GetTitle() string
	GetCommands() []Command
	GetOut() io.Writer // Used for testing, should be normally os.Stdout.
	GetErr() io.Writer // Used for testing, should be normally os.Stderr.
}

type Command interface {
	Run(a Application, args []string) int
	GetName() string
	GetUsageLine() string
	GetShortDesc() string
	GetLongDesc() string
	// Resets the flag state.
	InitFlags()
	GetFlags() *flag.FlagSet
}

type DefaultApplication struct {
	Name     string
	Title    string
	Commands []Command
}

func (a *DefaultApplication) GetName() string {
	return a.Name
}

func (a *DefaultApplication) GetTitle() string {
	return a.Title
}

func (a *DefaultApplication) GetCommands() []Command {
	return a.Commands
}

func (a *DefaultApplication) GetOut() io.Writer {
	return os.Stdout
}

func (a *DefaultApplication) GetErr() io.Writer {
	return os.Stderr
}

type DefaultCommand struct {
	UsageLine string
	ShortDesc string
	LongDesc  string
	Flag      *flag.FlagSet
}

func (c *DefaultCommand) GetUsageLine() string {
	return c.UsageLine
}

func (c *DefaultCommand) GetShortDesc() string {
	return c.ShortDesc
}

func (c *DefaultCommand) GetLongDesc() string {
	return c.LongDesc
}

func (c *DefaultCommand) GetFlags() *flag.FlagSet {
	if c.Flag == nil {
		panic("")
	}
	return c.Flag
}

// Name returns the command's name: the first word in the usage line.
func (c *DefaultCommand) GetName() string {
	name := c.GetUsageLine()
	i := strings.Index(name, " ")
	if i >= 0 {
		name = name[:i]
	}
	return name
}

func usage(out io.Writer, a Application) {
	usageTemplate := `{{.GetTitle}}

Usage:  {{.GetName}} [command] [arguments]

Commands:{{range .Commands}}
    {{.GetName | printf "%-11s"}} {{.GetShortDesc}}{{end}}

Use "{{.GetName}} help [command]" for more information about a command.

`
	tmpl(out, usageTemplate, a)
}

func getCommandUsageHandler(out io.Writer, a Application, cmd Command, helpUsed *bool) func() {
	return func() {
		helpTemplate := "{{.Cmd.GetLongDesc | trim | wrapWithLines}}usage:  {{.App.GetName}} {{.Cmd.GetUsageLine}}\n"
		dict := struct {
			App Application
			Cmd Command
		}{a, cmd}
		tmpl(out, helpTemplate, dict)
		cmd.GetFlags().PrintDefaults()
		*helpUsed = true
	}
}

// Initialize commands.
func initCommands(a Application, out io.Writer, helpUsed *bool) {
	for _, cmd := range a.GetCommands() {
		cmd.InitFlags()
		cmd.GetFlags().Usage = getCommandUsageHandler(out, a, cmd, helpUsed)
		cmd.GetFlags().SetOutput(out)
		cmd.GetFlags().Init(cmd.GetName(), flag.ContinueOnError)
	}
}

// Finds a command by name.
func FindCommand(a Application, name string) Command {
	for _, cmd := range a.GetCommands() {
		if cmd.GetName() == name {
			return cmd
		}
	}
	return nil
}

// Runs the application, scheduling the subcommand.
func Run(a Application, args []string) int {
	var helpUsed bool
	initCommands(a, a.GetErr(), &helpUsed)

	// Process general flags first, mainly for -help.
	flag.Usage = func() {
		usage(a.GetErr(), a)
		helpUsed = true
	}

	// Defaults; do not parse during unit tests because flag.commandLine.errorHandling == ExitOnError. :(
	if args == nil {
		flag.Parse()
		args = flag.Args()
	}

	if len(args) < 1 {
		// Need a command.
		usage(a.GetErr(), a)
		return 2
	}

	if cmd := FindCommand(a, args[0]); cmd != nil {
		cmd.GetFlags().Parse(args[1:])
		if helpUsed {
			return 0
		}
		return cmd.Run(a, cmd.GetFlags().Args())
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

type help struct {
	DefaultCommand
}

var cmdHelp = &help{
	DefaultCommand{
		UsageLine: "help <command>",
		ShortDesc: "prints help about a command",
		LongDesc:  "Prints an overview of every commands or information about a specific command.",
	},
}

func (c *help) InitFlags() {
	c.Flag = &flag.FlagSet{}
}

func (c *help) Run(a Application, args []string) int {
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
	initCommands(a, a.GetOut(), &helpUsed)

	if cmd := FindCommand(a, args[0]); cmd != nil {
		cmd.GetFlags().Usage()
		return 0
	}

	fmt.Fprintf(a.GetErr(), "%s: unknown command %#q\n\nRun '%s help' for usage.\n", a.GetName(), args[0], a.GetName())
	return 2
}
