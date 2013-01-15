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
	"bytes"
	"github.com/maruel/subcommands/subcommandstest"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
	"time"
)

type mockCache struct {
	*subcommandstest.TB
	root     *EntryCache
	closed   bool
	creation []byte
}

func (c *mockCache) Root() *EntryCache {
	c.Assertf(c.closed == false, "Was unexpectedly closed")
	return c.root
}

func (c *mockCache) Close() {
	c.Assertf(c.closed == false, "Was unexpectedly closed")
	c.closed = false
}

func (a *DumbcasAppMock) LoadCache() (Cache, error) {
	//return loadCache()
	if a.cache == nil {
		a.cache = &mockCache{a.TB, &EntryCache{}, false, debug.Stack()}
	} else {
		a.Assertf(a.cache.closed == true, "Was not closed properly; %s", a.cache.creation)
		a.cache.closed = false
	}
	return a.cache, nil
}

func TestCacheNormal(t *testing.T) {
	// Just makes sure loading the real cache doesn't crash.
	t.Parallel()
	tb := subcommandstest.MakeTB(t)
	cache, err := loadCache(tb.GetLog())
	tb.Assertf(err == nil, "Oops")
	defer cache.Close()
	tb.Assertf(cache.Root() != nil, "Oops")
}

func TestCachePath(t *testing.T) {
	t.Parallel()
	tb := subcommandstest.MakeTB(t)
	p, err := getCachePath()
	tb.Assertf(err == nil, "Oops")
	tb.Assertf(filepath.IsAbs(p), "Oops")
}

func TestCacheRedirected(t *testing.T) {
	t.Parallel()
	tb := subcommandstest.MakeTB(t)
	tempData := makeTempDir(tb, "cache")
	defer removeTempDir(tempData)
	load := func() (Cache, error) {
		return loadCacheInner(tempData, tb.GetLog())
	}
	testCacheImpl(tb, load)
}

func TestCacheMock(t *testing.T) {
	t.Parallel()
	tb := subcommandstest.MakeTB(t)
	// Keep the cache alive, since it's all in-memory.
	mock := &mockCache{tb, &EntryCache{}, false, nil}
	load := func() (Cache, error) {
		return mock, nil
	}
	testCacheImpl(tb, load)
}

func testCacheImpl(t *subcommandstest.TB, load func() (Cache, error)) {
	now := time.Now().UTC().Unix()
	{
		c, err := load()
		t.Assertf(err == nil, "Failed to create cache", err)
		if c.Root().CountMembers() != 1 {
			c.Root().Print(os.Stderr, "")
			t.Fatalf("Oops: %d", c.Root().CountMembers())
		}
		if c.Root().Files != nil {
			c.Root().Print(os.Stderr, "")
			t.Fatalf("Oops: %d", c.Root().CountMembers())
		}
		i := FindInCache(c, filepath.Join("foo", "bar"))
		i.Sha1 = "x"
		i.Size = 1
		i.Timestamp = 2
		i.LastTested = now
		c.Close()
	}
	{
		c, err := load()
		t.Assertf(err == nil, "Failed to create cache", err)
		b := &bytes.Buffer{}
		c.Root().Print(b, "")
		t.Assertf(b.String() == "- 'foo'\n  - 'bar'\n    Sha1: x\n    Size: 1\n", "Unexpected: %s", b.String())
		foo := c.Root().Files["foo"]
		bar := foo.Files["bar"]
		if bar.Sha1 != "x" || bar.Size != 1 || bar.Timestamp != 2 || bar.LastTested != now {
			t.Assertf(false, "Oops: %d", c.Root().CountMembers())
		}
		c.Close()
	}
}
