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
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

var cmdArchive = &Command{
	UsageLine: "archive <.toArchive> -out <out>",
	ShortDesc: "archive files to a dumbcas archive",
	LongDesc:  "Archives files listed in <.toArchive> file to a directory in the DumbCas(tm) layout. Files listed may be in relative path or in absolute path and may contain environment variables.",
	CommandRun: func() CommandRun {
		c := &archiveRun{}
		c.Init()
		c.Flags.StringVar(&c.comment, "comment", "", "Comment to embed in the file")
		return c
	},
}

type archiveRun struct {
	CommonFlags
	comment string
}

// For an item, tries to refresh its sha1 efficiently.
func updateFile(cache *EntryCache, item inputItem) (bool, error) {
	now := time.Now().Unix()
	size := item.Size()
	timestamp := item.ModTime().Unix()
	// If the file already exist, check for the timestamp and size to match.
	if cache.Size == size && cache.Timestamp == timestamp {
		cache.LastTested = now
		return false, nil
	}

	digest, err := sha1FilePath(item.fullPath)
	if err != nil {
		return false, err
	}
	cache.Sha1 = digest
	cache.Size = size
	cache.Timestamp = timestamp
	cache.LastTested = now
	return true, nil
}

// Reads a file with each line as an entry in the slice.
func readFileAsStrings(filepath string) ([]string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("Failed to read %s: %s", filepath, err)
	}
	b := bufio.NewReader(f)
	lines := []string{}
	for {
		line, err := b.ReadString('\n')
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			err = fmt.Errorf("Failed to read %s: %s", filepath, err)
			break
		}
	}
	return lines, err
}

type syncInt int64

func (s *syncInt) Add(i int64) {
	atomic.AddInt64((*int64)(s), i)
}

func (s *syncInt) Get() int64 {
	return atomic.LoadInt64((*int64)(s))
}

func (s *syncInt) g() syncInt {
	return syncInt(s.Get())
}

// Statistics are used with atomic functions. While not Go-idiomatic, it's much
// faster than using mutexes when calling i++ 100k times.
type StatsValues struct {
	errors           syncInt
	found            syncInt // enumerateInputs()
	totalSize        syncInt
	nbHashed         syncInt // hashInputs()
	bytesHashed      syncInt
	nbNotHashed      syncInt
	bytesNotHashed   syncInt
	nbArchived       syncInt // archiveInputs()
	bytesArchived    syncInt
	nbNotArchived    syncInt
	bytesNotArchived syncInt
}

// Stores statistic of the on-going process.
type Stats struct {
	StatsValues
	interrupted syncInt
	out         chan<- string
	done        chan<- bool
}

// Creates a copy of StatsValues. Note that the copy *may* be inconsistent.
func (s *StatsValues) Copy() *StatsValues {
	return &StatsValues{
		s.errors.g(),
		s.found.g(),
		s.totalSize.g(),
		s.nbHashed.g(),
		s.bytesHashed.g(),
		s.nbNotHashed.g(),
		s.bytesNotHashed.g(),
		s.nbArchived.g(),
		s.bytesArchived.g(),
		s.nbNotArchived.g(),
		s.bytesNotArchived.g(),
	}
}

// Compares two local copy of StatsValues. Must *not* be used on a Stats instance.
func (lhs *StatsValues) Equals(rhs *StatsValues) bool {
	return (lhs.errors.Get() == rhs.errors.Get() &&
		lhs.found.Get() == rhs.found.Get() &&
		lhs.totalSize.Get() == rhs.totalSize.Get() &&
		lhs.nbHashed.Get() == rhs.nbHashed.Get() &&
		lhs.bytesHashed.Get() == rhs.bytesHashed.Get() &&
		lhs.nbNotHashed.Get() == rhs.nbNotHashed.Get() &&
		lhs.bytesNotHashed.Get() == rhs.bytesNotHashed.Get() &&
		lhs.nbArchived.Get() == rhs.nbArchived.Get() &&
		lhs.bytesArchived.Get() == rhs.bytesArchived.Get() &&
		lhs.nbNotArchived.Get() == rhs.nbNotArchived.Get() &&
		lhs.bytesNotArchived.Get() == rhs.bytesNotArchived.Get())
}

type inputItem struct {
	fullPath string
	relPath  string
	os.FileInfo
}

// enumerateInputs reads the directories trees of each inputs and send each
// file into the output channel.
func (s *Stats) enumerateInputs(inputs []string) <-chan inputItem {
	// Throtttle after 128k entries.
	c := make(chan inputItem, 128000)
	go func() {
		start := time.Now().UTC()
		defer func() {
			close(c)
			s.done <- true
		}()

		// Do each entry serially. In theory there would be marginal gain by doing
		// them concurrently if the inputs are on different drives but for the
		// common use case where it's multiple directories on a single disk-based
		// HD, it's going to be slower.
		for _, input := range inputs {
			stat, err := os.Stat(input)
			if err != nil {
				// Eat the error and continue archiving other items.
				s.errors.Add(1)
				s.out <- fmt.Sprintf("Failed to process %s: %s", input, err)
				continue
			}
			if stat.IsDir() {
				// Send the items back in the channel.
				d := EnumerateTree(input)
				cont := true
				for cont {
					select {
					case <-InterruptedChannel:
						// Early exit.
						s.interrupted.Add(1)
						return
					case item, ok := <-d:
						if !ok {
							// Move on the next item.
							cont = false
							continue
						}
						if item.Error != nil {
							// Eat the error and continue archiving other items.
							s.errors.Add(1)
							s.out <- fmt.Sprintf("Failed to process %s: %s", input, err)
						} else if !item.IsDir() {
							// Ignores directories. This tool is backing up content, not
							// directories.
							s.found.Add(1)
							s.totalSize.Add(item.Size())
							// TODO(maruel): Not necessarily true?
							relPath := item.FullPath[len(input)+1:]
							//s.out <- fmt.Sprintf("%s: %d", relPath, item.Size())
							c <- inputItem{item.FullPath, relPath, item.FileInfo}
						}
					}
				}
			} else {
				s.found.Add(1)
				s.totalSize.Add(stat.Size())
				relPath := path.Base(input)
				c <- inputItem{input, relPath, stat}
			}
		}
		end := time.Now().UTC()
		s.out <- fmt.Sprintf("Done enumerating inputs: %s", end.Sub(start).String())
	}()
	return c
}

type itemToArchive struct {
	fullPath string
	relPath  string
	sha1     string
	size     int64
}

// Calculates each entry. Assumes inputs is cleaned paths.
func (s *Stats) hashInputs(a DumbcasApplication, inputs <-chan inputItem) <-chan itemToArchive {
	c := make(chan itemToArchive, 4096)
	go func() {
		// LoadCache must return a valid Cache instance even in case of failure.
		cache, err := a.LoadCache()
		if err != nil {
			s.out <- fmt.Sprintf("Failed to load cache: %s\nWARNING: It will be unbearably slow!", err)
		}
		defer func() {
			// Must save the cache *before* sending the 'done' signal.
			close(c)
			cache.Close()
			s.done <- true
		}()
		for {
			select {
			case <-InterruptedChannel:
				// Early exit.
				s.interrupted.Add(1)
				return
			case item, ok := <-inputs:
				if !ok {
					s.out <- fmt.Sprintf("Done hashing.")
					return
				}
				if item.IsDir() {
					panic("This can't happen; enumerateInputs() should eat all the directories.")
				}
				size := item.Size()
				cachedItem := FindInCache(cache, item.fullPath)
				if wasHashed, err := updateFile(cachedItem, item); err != nil {
					// Eat the error and continue archiving other items.
					s.errors.Add(1)
					s.out <- fmt.Sprintf("Failed to process %s: %s", item.fullPath, err)
					continue
				} else if wasHashed {
					//s.out <- fmt.Sprintf("Hashed: %s", item.relPath)
					s.nbHashed.Add(1)
					s.bytesHashed.Add(size)
				} else {
					s.nbNotHashed.Add(1)
					s.bytesNotHashed.Add(size)
				}
				c <- itemToArchive{item.fullPath, item.relPath, cachedItem.Sha1, size}
			}
		}
	}()
	return c
}

// Archives one item in the CAS table.
func (s *Stats) archiveItem(item itemToArchive, cas CasTable) {
	f, err := os.Open(item.fullPath)
	if err != nil {
		s.errors.Add(1)
		s.out <- fmt.Sprintf("Failed to archive %s: %s", item.fullPath, err)
		return
	}
	defer f.Close()
	err = cas.AddEntry(f, item.sha1)
	if os.IsExist(err) {
		s.nbNotArchived.Add(1)
		s.bytesNotArchived.Add(item.size)
	} else if err == nil {
		s.nbArchived.Add(1)
		s.bytesArchived.Add(item.size)
	} else {
		s.errors.Add(1)
		s.out <- fmt.Sprintf("Failed to archive %s: %s", item.fullPath, err)
	}
}

// Creates the Entry instance and the necessary Entry tree for |item|.
func makeEntry(root *Entry, item itemToArchive) {
	for _, p := range strings.Split(item.relPath, string(filepath.Separator)) {
		if root.Files == nil {
			root.Files = make(map[string]*Entry)
		}
		if root.Files[p] == nil {
			root.Files[p] = &Entry{}
		}
		root = root.Files[p]
	}
	root.Sha1 = item.sha1
	root.Size = item.size
}

// Archives the items.
func (s *Stats) archiveInputs(a DumbcasApplication, cas CasTable, items <-chan itemToArchive) <-chan string {
	c := make(chan string)
	go func() {
		defer func() {
			close(c)
			s.done <- true
		}()
		entryRoot := &Entry{}
		cont := true
		for cont {
			select {
			case <-InterruptedChannel:
				// Early exit.
				s.interrupted.Add(1)
				return
			case item, ok := <-items:
				if !ok {
					cont = false
					continue
				}
				//s.out <- fmt.Sprintf("Archiving: %s", item.relPath)
				makeEntry(entryRoot, item)
				s.archiveItem(item, cas)
			}
		}
		// Serializes the entry file to archive it too.
		data, err := json.Marshal(entryRoot)
		if err != nil {
			s.errors.Add(1)
			s.out <- fmt.Sprintf("Failed to marshal entry file: %s", err)
		} else {
			entrySha1, err := AddBytes(cas, data)
			if os.IsExist(err) {
				s.nbNotArchived.Add(1)
				s.bytesNotArchived.Add(int64(len(data)))
				c <- entrySha1
			} else if err == nil {
				s.nbArchived.Add(1)
				s.bytesArchived.Add(int64(len(data)))
				c <- entrySha1
			} else {
				s.errors.Add(1)
				s.out <- fmt.Sprintf("Failed to archive entry file: %s", err)
			}
		}
	}()
	return c
}

// Converts to absolute paths and evaluate environment variables.
func cleanupList(relDir string, inputs []string) {
	for index, item := range inputs {
		item = os.ExpandEnv(item)
		item = strings.Replace(item, "/", string(filepath.Separator), 0)
		if !path.IsAbs(item) {
			item = path.Join(relDir, item)
		}
		inputs[index] = path.Clean(item)
	}
}

func toMb(i int64) float64 {
	return float64(i) / 1024. / 1024.
}

// Loads the list of inputs and starts the concurrent processes:
// - Enumerating the trees.
// - Updating the hash for each items in the cache.
// - Archiving items.
func (c *archiveRun) main(a DumbcasApplication, toArchiveArg string) error {
	if err := c.Parse(a, true); err != nil {
		return err
	}

	toArchive, err := filepath.Abs(toArchiveArg)
	if err != nil {
		return fmt.Errorf("Failed to process %s", toArchiveArg)
	}

	inputs, err := readFileAsStrings(toArchive)
	if err != nil {
		return err
	}
	// Make sure the file itself is archived too.
	inputs = append(inputs, toArchive)
	a.GetLog().Printf("Found %d entries to backup in %s", len(inputs), toArchive)
	cleanupList(path.Dir(toArchive), inputs)

	// Start the processes.
	output := make(chan string)
	done := make(chan bool, 3)
	s := Stats{out: output, done: done}
	items_to_scan := s.enumerateInputs(inputs)
	items_hashed := s.hashInputs(a, items_to_scan)
	entry := s.archiveInputs(a, c.cas, items_hashed)

	headerWasPrinted := false
	columns := []string{
		"Found",
		"Hashed",
		"In cache",
		"Archived",
		"Skipped",
		"Done",
	}
	for i, _ := range columns {
		columns[i] = fmt.Sprintf("%-19s", columns[i])
	}
	column := strings.TrimSpace(strings.Join(columns, ""))

	errDone := errors.New("Dummy")
	prevStats := s.Copy()
	for err == nil {
		select {
		case line := <-output:
			a.GetLog().Print(line)
		case <-InterruptedChannel:
			// Early exit. Note this as an error.
			err = fmt.Errorf("Was interrupted.")
		case item, ok := <-entry:
			if !ok {
				e := s.errors.Get()
				if e != 0 {
					err = fmt.Errorf("Got %d errors!", e)
				} else if s.interrupted.Get() != 0 {
					err = fmt.Errorf("Was interrupted.")
				} else {
					err = fmt.Errorf("Unexpected error.")
				}
				continue
			}
			if item != "" {
				node := &Node{Entry: item, Comment: c.comment}
				_, err = c.nodes.AddEntry(node, path.Base(toArchive))
				err = errDone
			} else {
				e := s.errors.Get()
				if e != 0 {
					err = fmt.Errorf("Got %d errors!", e)
				} else if s.interrupted.Get() != 0 {
					err = fmt.Errorf("Was interrupted.")
				} else {
					err = fmt.Errorf("Unexpected error.")
				}
			}
		case <-time.After(5 * time.Second):
			nextStats := s.Copy()
			if !prevStats.Equals(nextStats) {
				if !headerWasPrinted {
					a.GetLog().Printf(column)
					headerWasPrinted = true
				}
				prevStats = nextStats
				fractionDone := float64(prevStats.bytesArchived.Get()+prevStats.bytesNotArchived.Get()) / float64(prevStats.totalSize.Get())
				a.GetLog().Printf(
					"%6d(%8.1fmb) %6d(%8.1fmb) %6d(%8.1fmb) %6d(%8.1fmb) %6d(%8.1fmb) %3.1f%% %d errors",
					prevStats.found.Get(),
					toMb(prevStats.totalSize.Get()),
					prevStats.nbHashed.Get(),
					toMb(prevStats.bytesHashed.Get()),
					prevStats.nbNotHashed.Get(),
					toMb(prevStats.bytesNotHashed.Get()),
					prevStats.nbArchived.Get(),
					toMb(prevStats.bytesArchived.Get()),
					prevStats.nbNotArchived.Get(),
					toMb(prevStats.bytesNotArchived.Get()),
					100.*fractionDone,
					prevStats.errors.Get())
			}
		}
	}
	if err == errDone {
		err = nil
	}
	if IsInterrupted() {
		fmt.Fprintf(a.GetOut(), "Was interrupted, waiting for processes to terminate.\n")
	}
	// Make sure all the worker threads are done. They may still be processing in
	// case of interruption.
	for i := 0; i < 3; i++ {
		<-done
	}
	fmt.Fprintf(a.GetOut(), column+"\n")
	fractionDone := float64(s.bytesArchived.Get()+s.bytesNotArchived.Get()) / float64(s.totalSize.Get())
	fmt.Fprintf(
		a.GetOut(),
		"%7d(%7.1fmb) %7d(%7.1fmb) %7d(%7.1fmb) %7d(%7.1fmb) %7d(%7.1fmb) %3.1f%% %d errors\n",
		s.found.Get(),
		toMb(s.totalSize.Get()),
		s.nbHashed.Get(),
		toMb(s.bytesHashed.Get()),
		s.nbNotHashed.Get(),
		toMb(s.bytesNotHashed.Get()),
		s.nbArchived.Get(),
		toMb(s.bytesArchived.Get()),
		s.nbNotArchived.Get(),
		toMb(s.bytesNotArchived.Get()),
		100.*fractionDone,
		s.errors.Get())
	return nil
}

func (c *archiveRun) Run(a Application, args []string) int {
	if len(args) != 1 {
		fmt.Fprintf(a.GetErr(), "%s: Must only provide a .toArchive file.\n", a.GetName())
		return 1
	}
	HandleCtrlC()
	d := a.(DumbcasApplication)
	if err := c.main(d, args[0]); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}
