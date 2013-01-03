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
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
	"time"
)

type mockCache struct {
	root     *EntryCache
	closed   bool
	t        *testing.T
	creation []byte
	log      *log.Logger
}

func (c *mockCache) Root() *EntryCache {
	if c.closed == true {
		c.t.Fatal("Was unexpectedly closed")
	}
	return c.root
}

func (c *mockCache) Close() {
	if c.closed == true {
		c.t.Fatal("Was unexpectedly closed")
	}
	c.closed = false
}

func (a *DumbcasAppMock) LoadCache() (Cache, error) {
	//return loadCache()
	if a.cache == nil {
		a.cache = &mockCache{&EntryCache{}, false, a.T, debug.Stack(), a.log}
	} else {
		if a.cache.closed {
			a.Fatalf("Was not closed properly; %s", a.cache.creation)
		}
		a.cache.closed = false
	}
	return a.cache, nil
}

func TestCacheNormal(t *testing.T) {
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

func TestCachePath(t *testing.T) {
	t.Parallel()
	p, err := getCachePath()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(p) {
		t.Fatal(p)
	}
}

func TestCacheRedirected(t *testing.T) {
	t.Parallel()
	tempData := makeTempDir(t, "cache")
	defer removeTempDir(tempData)
	load := func() (Cache, error) {
		return loadCacheInner(tempData)
	}
	testCacheImpl(t, load)
}

func TestCacheMock(t *testing.T) {
	t.Parallel()
	log := getLog(false)
	mock := &mockCache{&EntryCache{}, false, t, nil, log}
	load := func() (Cache, error) {
		return mock, nil
	}
	testCacheImpl(t, load)
}

func testCacheImpl(t *testing.T, load func() (Cache, error)) {
	now := time.Now().UTC().Unix()
	{
		c, err := load()
		if err != nil {
			t.Fatalf("Failed to create cache", err)
		}
		if c.Root().CountMembers() != 1 {
			c.Root().Print(os.Stderr, "")
			t.Fatalf("Oops: %d", c.Root().CountMembers())
		}
		if c.Root().Files != nil {
			c.Root().Print(os.Stderr, "")
			t.Fatalf("Oops: %d", c.Root().CountMembers())
		}
		c.Root().Files = make(map[string]*EntryCache)
		c.Root().Files["foo"] = &EntryCache{Sha1: "x", Size: 1, Timestamp: 2, LastTested: now}
		c.Close()
	}
	{
		c, err := load()
		if err != nil {
			t.Fatalf("Failed to create cache", err)
		}
		if c.Root().CountMembers() != 2 {
			c.Root().Print(os.Stderr, "")
			t.Fatalf("Oops: %d", c.Root().CountMembers())
		}
		foo := c.Root().Files["foo"]
		if foo.Sha1 != "x" || foo.Size != 1 || foo.Timestamp != 2 || foo.LastTested != now {
			c.Root().Print(os.Stderr, "")
			t.Fatalf("Oops: %d", c.Root().CountMembers())
		}
		c.Close()
	}
}
