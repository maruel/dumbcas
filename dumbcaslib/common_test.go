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
	"io/ioutil"
	"os"
	"testing"

	"github.com/maruel/ut"
)

func makeTempDir(t testing.TB, name string) string {
	name, err := ioutil.TempDir("", "dumbcas_"+name)
	ut.AssertEqual(t, nil, err)
	return name
}

func removeDir(t testing.TB, tempDir string) {
	err := os.RemoveAll(tempDir)
	ut.AssertEqual(t, nil, err)
}
