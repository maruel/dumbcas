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
	"github.com/maruel/subcommands"
	"io"
	"log"
	"os"
	"path/filepath"
)

var cmdRestore = &subcommands.Command{
	UsageLine: "restore <node> -out <out>",
	ShortDesc: "restores a tree from a dumbcas archive",
	LongDesc:  "Restores files listed in <node> archive to a directory from a DumbCas(tm) archive.",
	CommandRun: func() subcommands.CommandRun {
		c := &restoreRun{}
		c.Init()
		c.Flags.StringVar(&c.Out, "out", "", "Directory to restore data to; required.")
		return c
	},
}

type restoreRun struct {
	CommonFlags
	Out string
}

// Restores entries and keep going on in case of error. Returns the first seen
// error.
// Do not overwrite files. A file already present is considered an error.
func restoreEntry(l *log.Logger, cas CasTable, entry *Entry, root string) (count int, out error) {
	if entry.Sha1 != "" {
		f, err := cas.Open(entry.Sha1)
		if err != nil {
			out = fmt.Errorf("Failed to fetch %s for %s: %s", entry.Sha1, root, err)
		} else {
			defer f.Close()
			baseDir := filepath.Dir(root)
			if err = os.MkdirAll(baseDir, 0755); err != nil && !os.IsExist(err) {
				out = fmt.Errorf("Failed to create %s: %s", baseDir, err)
			} else {
				dst, err := os.OpenFile(root, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
				if err != nil {
					out = fmt.Errorf("Failed to create %s in %s: %s", root, baseDir, err)
				} else {
					size, err := io.Copy(dst, f)
					if err != nil {
						out = fmt.Errorf("Failed to copy %s: %s", root, err)
					} else if size != entry.Size {
						out = fmt.Errorf("Failed to write %s, expected %d, wrote %d", root, entry.Size, size)
					} else {
						count += 1
					}
				}
			}
		}
		if out != nil {
			l.Printf("%s(%d): %s", root, entry.Size, out)
		} else {
			l.Printf("%s(%d)", root, entry.Size)
		}
	}
	for name, child := range entry.Files {
		c, err := restoreEntry(l, cas, child, filepath.Join(root, name))
		if err != nil && out == nil {
			out = err
		}
		count += c
	}
	return
}

func (c *restoreRun) main(a DumbcasApplication, nodeArg string) error {
	if err := c.Parse(a, true); err != nil {
		return err
	}

	// Load the Node and process it.
	// Do it serially for now, assuming that it is I/O bound on magnetic disks.
	// For a network CAS, it would be good to implement concurrent fetches.

	f, err := c.nodes.Open(nodeArg)
	if err != nil {
		return err
	}
	defer f.Close()
	node := &Node{}
	if err := loadReaderAsJson(f, node); err != nil {
		return err
	}

	entry, err := LoadEntry(c.cas, node.Entry)
	if err != nil {
		return err
	}
	// TODO(maruel): Progress bar.
	count, err := restoreEntry(a.GetLog(), c.cas, entry, c.Out)
	fmt.Fprintf(a.GetOut(), "Restored %d files in %s\n", count, c.Out)
	return err
}

func (c *restoreRun) Run(a subcommands.Application, args []string) int {
	if len(args) != 1 {
		fmt.Fprintf(a.GetErr(), "%s: Must only provide a <node>.\n", a.GetName())
		return 1
	}
	HandleCtrlC()
	d := a.(DumbcasApplication)
	if err := c.main(d, args[0]); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}
