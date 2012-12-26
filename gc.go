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
	"log"
)

var cmdGc = &Command{
	Run:       runGc,
	UsageLine: "gc",
	ShortDesc: "moves to trash all objects that are not referenced anymore",
	LongDesc:  "Scans each node and each entry file to determine if each cas entry is referenced or not.",
	Flag:      GetCommonFlags(),
}

func TagRecurse(entries map[string]bool, entry *Entry) {
	if entry.Sha1 != "" {
		entries[entry.Sha1] = true
	}
	for _, i := range entry.Files {
		TagRecurse(entries, i)
	}
}

func gcMain(name string, l *log.Logger) error {
	cas, err := CommonFlag(false, false)
	if err != nil {
		return err
	}

	nodes, err := LoadNodesTable(Root, cas, l)
	if err != nil {
		return err
	}
	entries := map[string]bool{}
	for item := range cas.Enumerate() {
		if item.Error != nil {
			// TODO(maruel): Leaks channel.
			cas.NeedFsck()
			return fmt.Errorf("Failed enumerating the CAS table %s", item.Error)
		}
		entries[item.Item] = false
	}
	l.Printf("Found %d entries", len(entries))

	// Load all the nodes.
	for item := range nodes.Enumerate() {
		if item.Error != nil {
			// TODO(maruel): Leaks channel.
			return item.Error
		}
		entries[item.Node.Entry] = true
		TagRecurse(entries, item.Entry)
	}

	orphans := []string{}
	for entry, tagged := range entries {
		if !tagged {
			orphans = append(orphans, entry)
		}
	}
	l.Printf("Found %d orphan", len(orphans))
	for _, orphan := range orphans {
		if err := cas.Remove(orphan); err != nil {
			cas.NeedFsck()
			return fmt.Errorf("Internal error while removing %s: %s", orphan, err)
		}
	}
	return nil
}

func runGc(a *Application, cmd *Command, args []string) int {
	if len(args) != 0 {
		fmt.Fprintf(a.Err, "%s: Unsupported arguments.\n", a.Name)
		return 1
	}
	if err := gcMain(a.Name, a.Log); err != nil {
		fmt.Fprintf(a.Err, "%s: %s\n", a.Name, err)
		return 1
	}
	return 0
}
