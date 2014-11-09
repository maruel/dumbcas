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
	"io"
	"path/filepath"
	"sort"

	"github.com/maruel/interrupt"
	"github.com/maruel/subcommands"
)

var cmdInfo = &subcommands.Command{
	UsageLine: "info <node>",
	ShortDesc: "prints information about a node",
	LongDesc:  "Prints the files listed in <node> archive from a DumbCas(tm) archive.",
	CommandRun: func() subcommands.CommandRun {
		c := &infoRun{}
		c.Init()
		return c
	},
}

type infoRun struct {
	CommonFlags
}

func printEntry(out io.Writer, entry *Entry, relPath string) (count int) {
	if entry.Sha1 != "" {
		fmt.Fprintf(out, " %s(%d)\n", relPath, entry.Size)
		count += 1
	}
	names := make([]string, 0, len(entry.Files))
	for name := range entry.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		child := entry.Files[name]
		c := printEntry(out, child, filepath.Join(relPath, name))
		count += c
	}
	return
}

func (c *infoRun) main(a DumbcasApplication, nodeArg string) error {
	if err := c.Parse(a, true); err != nil {
		return err
	}

	// Load the Node and process it.
	f, err := c.nodes.Open(nodeArg)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()
	node := &Node{}
	if err := loadReaderAsJson(f, node); err != nil {
		return err
	}

	entry, err := LoadEntry(c.cas, node.Entry)
	if err != nil {
		return err
	}

	count := printEntry(a.GetOut(), entry, "")
	fmt.Fprintf(a.GetOut(), "Total %d\n", count)
	return nil
}

func (c *infoRun) Run(a subcommands.Application, args []string) int {
	if len(args) != 1 {
		fmt.Fprintf(a.GetErr(), "%s: Must only provide a <node>.\n", a.GetName())
		return 1
	}
	interrupt.HandleCtrlC()
	d := a.(DumbcasApplication)
	if err := c.main(d, args[0]); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}
