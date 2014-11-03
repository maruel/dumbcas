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
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
)

// LoadCache loads the cache from ~/.dumbcas/cache.json and keeps it open until
// the call to Close(). It is guaranteed to return a non-nil Cache instance even
// in case of failure to load the cache from disk and that error is non-nil.
//
// TODO(maruel): Ensure proper file locking. One way is to always create a new
// file when adding data and then periodically garbage-collect the files.
func LoadCache() (Cache, error) {
	cacheDir, err := getCachePath()
	if err != nil {
		return &cache{&EntryCache{}, ""}, err
	}
	return loadCacheInner(cacheDir)
}

func loadCacheInner(cacheDir string) (Cache, error) {
	cache := &cache{&EntryCache{}, filepath.Join(cacheDir, "cache.gob")}
	if err := os.Mkdir(cacheDir, 0700); err != nil && !os.IsExist(err) {
		return cache, fmt.Errorf("Failed to access %s: %s", cacheDir, err)
	}
	f, err := os.OpenFile(cache.filePath, os.O_RDONLY, 0600)
	if f == nil {
		if os.IsNotExist(err) {
			// Do not this as an error, it would be confusing.
			return cache, nil
		}
		return cache, fmt.Errorf("Failed to access %s: %s", cache.filePath, err)
	}
	defer func() {
		_ = f.Close()
	}()
	// The cache uses gob instead of json because:
	// - The data can be read and written incrementally instead of having to read
	//   it all at once.
	// - It's significantly faster than json.
	// - It's significantly smaller than json.
	// - The program works fine without cache so it's not a big deal if it ever
	//   become backward incomatible.
	d := gob.NewDecoder(f)
	if err = d.Decode(cache.root); err != nil && err != io.EOF {
		// Ignore unmarshaling failure by reseting the content. Better be safe than
		// sorry.
		err = fmt.Errorf("failed loading cache: %s", err)
		cache.root = &EntryCache{}
	}
	return cache, err
}

type cache struct {
	root     *EntryCache
	filePath string
}

func getCachePath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(usr.HomeDir, ".dumbcas"), nil
}

func (c *cache) Root() *EntryCache {
	return c.root
}

func encode(filePath string, root *EntryCache) error {
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if f == nil {
		return fmt.Errorf("failed to save cache %s: %s", filePath, err)
	}
	defer func() {
		_ = f.Close()
	}()
	// TODO(maruel): Trim anything > ~1yr old.
	e := gob.NewEncoder(f)
	if err := e.Encode(root); err != nil {
		return fmt.Errorf("failed to write %s: %s", filePath, err)
	}
	return nil
}

func (c *cache) Close() error {
	if c.filePath == "" {
		return nil
	}
	if err := encode(c.filePath, c.root); err != nil {
		return err
	}
	stat, err := os.Stat(c.filePath)
	if err != nil {
		return fmt.Errorf("Unexpected error while stat'ing %s: %s", c.filePath, err)
	} else if stat.Size() < 100 {
		return fmt.Errorf("Failed to serialize %s: %d", c.filePath, stat.Size())
	}
	return nil
}
