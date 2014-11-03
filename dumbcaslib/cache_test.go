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
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/maruel/ut"
)

func TestCacheNormal(t *testing.T) {
	// Just makes sure loading the real cache doesn't crash.
	t.Parallel()
	cache, err := LoadCache()
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
	tempData := makeTempDir(t, "cache")
	defer removeDir(t, tempData)
	load := func() (Cache, error) {
		return loadCacheInner(tempData)
	}
	testCacheImpl(t, load)
}

func TestFakeCache(t *testing.T) {
	t.Parallel()
	// Keep the cache alive, since it's all in-memory.
	fake := MakeMemoryCache()
	load := func() (Cache, error) {
		return fake, nil
	}
	testCacheImpl(t, load)
}

func testCacheImpl(t testing.TB, load func() (Cache, error)) {
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
