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

var cmdFsck = &Command{
	Run:       runFsck,
	UsageLine: "fsck",
	ShortDesc: "moves to trash all objects that are not valid content anymore",
	LongDesc:  "Recalculate the sha-1 of each dumbcas entry and remove any that are corrupted",
	Flag:      GetCommonFlags(),
}

func fsckMain() error {
	cas, err := CommonFlag(false, true)
	if err != nil {
		return err
	}

	// TODO(maruel): check nodes too!
	//nodesDir := path.Join(Root, NodesName)
	trash := MakeTrash(Root)

	c := make(chan Item)
	count := 0
	corrupted := 0
	invalid := 0
	go cas.Enumerate(c)
	for {
		item := <-c
		if item.Item != "" {
			count += 1
			f, err := cas.Open(item.Item)
			if err != nil {
				return fmt.Errorf("Failed to open %s: %s", item.Item, err)
			}
			defer f.Close()
			actual, err := sha1File(f)
			if err != nil {
				// Probably Disk error.
				return fmt.Errorf("Aborting! Failed to sha1 %s: %s", item.Item, err)
			}
			if actual != item.Item {
				corrupted += 1
				log.Printf("Found invalid object, %s != %s", item.Item, actual)
				if err := cas.Trash(item.Item, trash); err != nil {
					return fmt.Errorf("Failed to trash object %s: %s", item.Item, err)
				}
			}
		} else if item.Invalid != "" {
			// Move it to trash right away.
			invalid += 1
			trash.Move(item.Invalid)
		} else if item.Error != nil {
			return fmt.Errorf("Failed enumerating the CAS table %s", item.Error)
		} else {
			break
		}
	}
	log.Printf("Scanned %d entries; found %d corrupted, %d invalid.", count, corrupted, invalid)
	return nil
}

func runFsck(a *Application, cmd *Command, args []string) int {
	if len(args) != 0 {
		fmt.Fprintf(a.Err, "%s: Unsupported arguments.\n", a.Name)
		return 1
	}
	if err := fsckMain(); err != nil {
		fmt.Fprintf(a.Err, "%s: %s\n", a.Name, err)
		return 1
	}
	return 0
}
