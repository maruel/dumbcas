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
)

var application = &DefaultApplication{
	Name:  "dumbcas",
	Title: "Dumbcas is a simple Content Addressed Datastore to be used as a simple backup tool.",
	Commands: []*Command{
		cmdArchive,
		cmdFsck,
		cmdGc,
		cmdHelp,
		cmdWeb,
	},
}

type DumbcasApplication interface {
	Application
	GetLog() *log.Logger
	LoadCache() (Cache, error)
	MakeCasTable(rootDir string) (CasTable, error)
	LoadNodesTable(rootDir string, cas CasTable) (NodesTable, error)
}

type dumbapp struct {
	*DefaultApplication
	log *log.Logger
}

func (d *dumbapp) GetLog() *log.Logger {
	return d.log
}

func (d *dumbapp) LoadCache() (Cache, error) {
	return loadCache()
}

func (d *dumbapp) MakeCasTable(rootDir string) (CasTable, error) {
	return makeCasTable(rootDir)
}

func (d *dumbapp) LoadNodesTable(rootDir string, cas CasTable) (NodesTable, error) {
	return loadNodesTable(rootDir, cas, d.GetLog())
}

func main() {
	log.SetFlags(log.Lmicroseconds)
	d := &dumbapp{application, log.New(application.GetErr(), "", log.LstdFlags|log.Lmicroseconds)}
	os.Exit(Run(d, nil))
}
