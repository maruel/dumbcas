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
	"os"
	"path"
)

const trashName = "trash"

type Trash struct {
	rootDir  string
	trashDir string
	created  bool
}

func MakeTrash(rootDir string) *Trash {
	if !path.IsAbs(rootDir) {
		return nil
	}
	return &Trash{rootDir: rootDir, trashDir: path.Join(rootDir, trashName)}
}

func (t *Trash) Move(relPath string) error {
	if !t.created {
		if err := os.Mkdir(t.trashDir, 0750); err != nil && !os.IsExist(err) {
			return fmt.Errorf("Failed to create %s: %s", t.trashDir, err)
		} else if err == nil {
			log.Print("Created trash at " + t.trashDir)
		}
		t.created = true
	}
	log.Printf("Moving %s", path.Join(t.rootDir, relPath))
	relDir := path.Dir(relPath)
	if relDir != "." {
		dir := path.Join(t.trashDir, relDir)
		if err := os.MkdirAll(dir, 0750); err != nil && !os.IsExist(err) {
			return fmt.Errorf("Failed to create %s: %s", dir, err)
		}
		log.Print("Created trash subdir " + dir)
	}
	return os.Rename(path.Join(t.rootDir, relPath), path.Join(t.trashDir, relPath))
}
