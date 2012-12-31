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
)

type fsck struct {
	DefaultCommand
}

var cmdFsck = &fsck{
	DefaultCommand{
		UsageLine: "fsck",
		ShortDesc: "moves to trash all objects that are not valid content anymore",
		LongDesc:  "Recalculate the sha-1 of each dumbcas entry and remove any that are corrupted",
		Flag:      GetCommonFlags(),
	},
}

func fsckMain(a DumbcasApplication) error {
	cas, _, err := CommonFlag(a, false, true)
	if err != nil {
		return err
	}

	// TODO(maruel): check nodes too!
	count := 0
	corrupted := 0
	for item := range cas.Enumerate() {
		if item.Error != nil {
			// TODO(maruel): Leaks channel.
			return fmt.Errorf("Failed enumerating the CAS table %s", item.Error)
		}
		count += 1
		f, err := cas.Open(item.Item)
		if err != nil {
			// TODO(maruel): Leaks channel.
			return fmt.Errorf("Failed to open %s: %s", item.Item, err)
		}
		defer f.Close()
		actual, err := sha1File(f)
		if err != nil {
			// Probably Disk error.
			// TODO(maruel): Leaks channel.
			return fmt.Errorf("Aborting! Failed to sha1 %s: %s", item.Item, err)
		}
		if actual != item.Item {
			corrupted += 1
			a.GetLog().Printf("Found corrupted object, %s != %s", item.Item, actual)
			if err := cas.Remove(item.Item); err != nil {
				// TODO(maruel): Leaks channel.
				return fmt.Errorf("Failed to trash object %s: %s", item.Item, err)
			}
		}
	}
	a.GetLog().Printf("Scanned %d entries; found %d corrupted, %d invalid.", count, corrupted)
	return nil
}

func (cmd *fsck) Run(a Application, args []string) int {
	if len(args) != 0 {
		fmt.Fprintf(a.GetErr(), "%s: Unsupported arguments.\n", a.GetName())
		return 1
	}
	d := a.(DumbcasApplication)
	if err := fsckMain(d); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}
