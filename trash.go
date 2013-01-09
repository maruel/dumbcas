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
	"path/filepath"
)

const TrashName = "trash"

type trash struct {
	rootDir  string
	trashDir string
	created  bool
}

type Trash interface {
	Move(relPath string) error
}

func MakeTrash(rootDir string) Trash {
	if !filepath.IsAbs(rootDir) {
		return nil
	}
	return &trash{rootDir: rootDir, trashDir: filepath.Join(rootDir, TrashName)}
}

func (t *trash) Move(relPath string) error {
	log.Printf("Move(%s)", relPath)
	if !t.created {
		if err := os.Mkdir(t.trashDir, 0750); err != nil && !os.IsExist(err) {
			return fmt.Errorf("Failed to create %s: %s", t.trashDir, err)
		} else if err == nil {
			log.Print("Created trash at " + t.trashDir)
		}
		t.created = true
	}
	relDir := filepath.Dir(relPath)
	if relDir != "." {
		dir := filepath.Join(t.trashDir, relDir)
		if err := os.MkdirAll(dir, 0750); err != nil && !os.IsExist(err) {
			return fmt.Errorf("Failed to create %s: %s", dir, err)
		}
		log.Print("Created trash subdir " + dir)
	}
	return os.Rename(filepath.Join(t.rootDir, relPath), filepath.Join(t.trashDir, relPath))
}
