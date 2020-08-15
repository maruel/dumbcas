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
	"regexp"

	"github.com/maruel/dumbcas/dumbcaslib"
	"github.com/maruel/subcommands"
)

var cmdFsck = &subcommands.Command{
	UsageLine: "fsck",
	ShortDesc: "moves to trash all objects that are not valid content anymore",
	LongDesc:  "Recalculate the sha-1 of each dumbcas entry and remove any that are corrupted",
	CommandRun: func() subcommands.CommandRun {
		c := &fsckRun{}
		c.Init()
		return c
	},
}

type fsckRun struct {
	CommonFlags
}

func (c *fsckRun) main(a DumbcasApplication) error {
	if err := c.Parse(a, true); err != nil {
		return err
	}

	count := 0
	corrupted := 0
	for item := range c.cas.Enumerate() {
		if item.Error != nil {
			a.GetLog().Printf("While enumerating the CAS table: %s", item.Error)
			continue
		}
		count++
		f, err := c.cas.Open(item.Item)
		if err != nil {
			// TODO(maruel): Leaks channel.
			return fmt.Errorf("Failed to open %s: %s", item.Item, err)
		}
		defer func() {
			_ = f.Close()
		}()
		actual, err := sha1Reader(f)
		if err != nil {
			// Probably Disk error.
			// TODO(maruel): Leaks channel.
			return fmt.Errorf("Aborting! Failed to calcultate the sha1 of %s: %s. Please find a valid copy of your CAS table ASAP.", item.Item, err)
		}
		if actual != item.Item {
			corrupted++
			a.GetLog().Printf("Found corrupted object, %s != %s", item.Item, actual)
			if err := c.cas.Remove(item.Item); err != nil {
				// TODO(maruel): Leaks channel.
				return fmt.Errorf("Failed to trash object %s: %s", item.Item, err)
			}
		}
	}
	a.GetLog().Printf("Scanned %d entries in CasTable; found %d corrupted.", count, corrupted)

	// TODO(maruel): Get the value from CasTable.
	hashLength := 40
	resha1 := regexp.MustCompile(fmt.Sprintf("^([a-f0-9]{%d})$", hashLength))
	count = 0
	corrupted = 0
	for item := range c.nodes.Enumerate() {
		// TODO(maruel): Can't differentiate between an I/O error or a corrupted node.
		// NodesTable.Enumerate() automatically clears corrupted nodes.
		// TODO(maruel): This is a layering error.
		if item.Error != nil {
			a.GetLog().Printf("While enumerating the Nodes table: %s", item.Error)
			continue
		}
		count++
		f, err := c.nodes.Open(item.Item)
		if err != nil {
			a.GetLog().Printf("Failed opening node %s: %s", item.Item, err)
			_ = c.nodes.Remove(item.Item)
			corrupted++
			continue
		}
		defer func() {
			_ = f.Close()
		}()
		node := &dumbcaslib.Node{}
		if err := dumbcaslib.LoadReaderAsJSON(f, node); err != nil {
			a.GetLog().Printf("Failed opening node %s: %s", item.Item, err)
			_ = c.nodes.Remove(item.Item)
			corrupted++
			continue
		}
		if !resha1.MatchString(node.Entry) {
			a.GetLog().Printf("Node %s is corrupted: %v", item.Item, node)
			_ = c.nodes.Remove(item.Item)
			corrupted++
			continue
		}
	}
	a.GetLog().Printf("Scanned %d entries in NodesTable; found %d corrupted.", count, corrupted)

	c.cas.ClearFsckBit()
	return nil
}

func (c *fsckRun) Run(a subcommands.Application, args []string, _ subcommands.Env) int {
	if len(args) != 0 {
		fmt.Fprintf(a.GetErr(), "%s: Unsupported arguments.\n", a.GetName())
		return 1
	}
	d := a.(DumbcasApplication)
	if err := c.main(d); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}
