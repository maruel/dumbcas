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
	"log"
	"runtime/debug"
	"testing"
)

type cacheMock struct {
	root     *EntryCache
	closed   bool
	t        *testing.T
	creation []byte
	log      *log.Logger
}

func (c *cacheMock) Root() *EntryCache {
	if c.closed == true {
		c.t.Fatal("Was unexpectedly closed")
	}
	return c.root
}

func (c *cacheMock) Close() {
	if c.closed == true {
		c.t.Fatal("Was unexpectedly closed")
	}
	c.closed = false
}

func (a *ApplicationMock) LoadCache() (Cache, error) {
	if a.cache == nil {
		a.cache = &cacheMock{&EntryCache{}, false, a.T, debug.Stack(), a.log}
	} else {
		if a.cache.closed {
			a.Fatalf("Was not closed properly; %s", a.cache.creation)
		}
		a.cache.closed = false
	}
	return a.cache, nil
}

func TestCache(t *testing.T) {
	t.Parallel()
	cache, err := loadCache()
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()
	if cache.Root() == nil {
		t.Fatal(err)
	}
}
