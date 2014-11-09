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
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
	"time"

	"github.com/maruel/subcommands/subcommandstest"
	"github.com/maruel/ut"
)

// A working Cache implementation that is very simple and keeps everything in
// memory.
type fakeCache struct {
	*subcommandstest.TB
	root     *EntryCache
	closed   bool
	creation []byte
}

func (c *fakeCache) Root() *EntryCache {
	ut.AssertEqualf(c, c.closed, false, "Was unexpectedly closed")
	return c.root
}

func (c *fakeCache) Close() {
	ut.AssertEqualf(c, c.closed, false, "Was unexpectedly closed")
	c.closed = false
}

func (a *DumbcasAppMock) LoadCache() (Cache, error) {
	//return loadCache()
	if a.cache == nil {
		a.cache = &fakeCache{a.TB, &EntryCache{}, false, debug.Stack()}
	} else {
		ut.AssertEqualf(a.TB, true, a.cache.closed, "Was not closed properly; %s", a.cache.creation)
		a.cache.closed = false
	}
	return a.cache, nil
}

func TestCacheNormal(t *testing.T) {
	// Just makes sure loading the real cache doesn't crash.
	t.Parallel()
	tb := subcommandstest.MakeTB(t)
	cache, err := loadCache(tb.GetLog())
	ut.AssertEqual(t, nil, err)
	defer cache.Close()
	ut.AssertEqual(t, false, nil == cache.Root())
}

func TestCachePath(t *testing.T) {
	t.Parallel()
	p, err := getCachePath()
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, true, filepath.IsAbs(p))
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

func TestFakeCache(t *testing.T) {
	t.Parallel()
	tb := subcommandstest.MakeTB(t)
	// Keep the cache alive, since it's all in-memory.
	fake := &fakeCache{tb, &EntryCache{}, false, nil}
	load := func() (Cache, error) {
		return fake, nil
	}
	testCacheImpl(tb, load)
}

func testCacheImpl(t *subcommandstest.TB, load func() (Cache, error)) {
	now := time.Now().UTC().Unix()
	{
		c, err := load()
		ut.AssertEqual(t, nil, err)
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
		ut.AssertEqual(t, nil, err)
		b := &bytes.Buffer{}
		c.Root().Print(b, "")
		ut.AssertEqual(t, "- 'foo'\n  - 'bar'\n    Sha1: x\n    Size: 1\n", b.String())
		foo := c.Root().Files["foo"]
		bar := foo.Files["bar"]
		if bar.Sha1 != "x" || bar.Size != 1 || bar.Timestamp != 2 || bar.LastTested != now {
			t.Fatalf("Oops: %d", c.Root().CountMembers())
		}
		c.Close()
	}
}
