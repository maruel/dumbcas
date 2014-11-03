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

	"github.com/maruel/dumbcas/dumbcaslib"
	"github.com/maruel/subcommands"
	"github.com/maruel/subcommands/subcommandstest"
)

var application = &subcommands.DefaultApplication{
	Name:  "dumbcas",
	Title: "Dumbcas is a simple Content Addressed Datastore to be used as a simple backup tool.",
	Commands: []*subcommands.Command{
		cmdArchive,
		cmdFsck,
		cmdGc,
		subcommands.CmdHelp,
		cmdInfo,
		cmdRestore,
		cmdVersion,
		cmdWeb,
	},
}

// DumbcasApplication is the main application.
type DumbcasApplication interface {
	subcommandstest.Application
	// LoadCache must return a valid Cache instance even in case of failure.
	LoadCache() (dumbcaslib.Cache, error)
	MakeCasTable(rootDir string) (dumbcaslib.CasTable, error)
	LoadNodesTable(rootDir string, cas dumbcaslib.CasTable) (dumbcaslib.NodesTable, error)
}

type dumbapp struct {
	*subcommands.DefaultApplication
	log *log.Logger
}

// Implementes subcommandstest.Application.
func (d *dumbapp) GetLog() *log.Logger {
	return d.log
}

func (d *dumbapp) LoadCache() (dumbcaslib.Cache, error) {
	return dumbcaslib.LoadCache()
}

func (d *dumbapp) MakeCasTable(rootDir string) (dumbcaslib.CasTable, error) {
	return dumbcaslib.MakeLocalCasTable(rootDir)
}

func (d *dumbapp) LoadNodesTable(rootDir string, cas dumbcaslib.CasTable) (dumbcaslib.NodesTable, error) {
	return dumbcaslib.LoadLocalNodesTable(rootDir, cas)
}

func main() {
	log.SetFlags(log.Lmicroseconds)
	d := &dumbapp{application, log.New(application.GetErr(), "", log.LstdFlags|log.Lmicroseconds)}
	os.Exit(subcommands.Run(d, nil))
}
