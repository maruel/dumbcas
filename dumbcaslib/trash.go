/* Copyright 2012 Marc-Antoine Ruel. Licensed under the Apache License, Version
2.0 (the "License"); you may not use this file except in compliance with the
License.  You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0. Unless required by applicable law or
agreed to in writing, software distributed under the License is distributed on
an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
or implied. See the License for the specific language governing permissions and
limitations under the License. */

package dumbcaslib

import (
	"fmt"
	"os"
	"path/filepath"
)

const trashName = "trash"

type trashImpl struct {
	rootDir  string
	trashDir string
	created  bool
}

type trash interface {
	move(relPath string) error
}

func makeTrash(rootDir string) trash {
	if !filepath.IsAbs(rootDir) {
		return nil
	}
	return &trashImpl{rootDir: rootDir, trashDir: filepath.Join(rootDir, trashName)}
}

func (t *trashImpl) move(relPath string) error {
	if !t.created {
		if err := os.Mkdir(t.trashDir, 0750); err != nil && !os.IsExist(err) {
			return fmt.Errorf("Failed to create %s: %s", t.trashDir, err)
		}
		t.created = true
	}
	relDir := filepath.Dir(relPath)
	if relDir != "." {
		dir := filepath.Join(t.trashDir, relDir)
		if err := os.MkdirAll(dir, 0750); err != nil && !os.IsExist(err) {
			return fmt.Errorf("Failed to create %s: %s", dir, err)
		}
	}
	return os.Rename(filepath.Join(t.rootDir, relPath), filepath.Join(t.trashDir, relPath))
}
