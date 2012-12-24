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
	"testing"
)

type cacheMock struct {
	root   *EntryCache
	closed bool
	t      *testing.T
}

func (c *cacheMock) Root() *EntryCache {
	if c.closed == true {
		c.t.Fail()
	}
	return c.root
}

func (c *cacheMock) Close() {
	if c.closed == true {
		c.t.Fail()
	}
	c.closed = true
}

func MockCache(t *testing.T) {
	LoadCache = func() (Cache, error) {
		return &cacheMock{&EntryCache{}, false, t}, nil
	}
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
