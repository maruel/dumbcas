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
	"time"
)

// Traverse synchronously both the cache and the entry table.
func Recurse(cache *EntryCache, entry *Entry, item string) (*EntryCache, *Entry) {
	cache.LastTested = time.Now().UTC().Unix()
	if cache.Files == nil {
		cache.Files = map[string]*EntryCache{}
	}
	if entry.Files == nil {
		entry.Files = map[string]*Entry{}
	}
	if _, ok := cache.Files[item]; !ok {
		cache.Files[item] = &EntryCache{}
	}
	if _, ok := entry.Files[item]; !ok {
		entry.Files[item] = &Entry{}
	}
	return cache.Files[item], entry.Files[item]
}

// Creates the tree of EntryCache and Entry based on itemPath.
func RecursePath(cache *EntryCache, entry *Entry, itemPath string) (*EntryCache, *Entry) {
	if filepath.Separator == '/' && itemPath[0] == '/' {
		itemPath = itemPath[1:]
	}
	parts := strings.SplitN(itemPath, string(filepath.Separator), 2)
	cache, entry = Recurse(cache, entry, parts[0])
	if len(parts) == 2 && parts[1] != "" {
		cache, entry = RecursePath(cache, entry, parts[1])
	}
	return cache, entry
}

func UpdateFile(cache *EntryCache, entry *Entry, item TreeItem) error {
	now := time.Now().Unix()
	size := item.FileInfo.Size()
	timestamp := item.FileInfo.ModTime().Unix()
	// If the file already exist, check for the timestamp and size to match.
	if cache.Size == size && cache.Timestamp == timestamp {
		entry.Sha1 = cache.Sha1
		entry.Size = size
		cache.LastTested = now
		return nil
	}

	digest, err := sha1FilePath(item.FullPath)
	if err != nil {
		return err
	}
	cache.Sha1 = digest
	cache.Size = size
	cache.Timestamp = timestamp
	cache.LastTested = now
	entry.Sha1 = digest
	entry.Size = size
	return nil
}

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

// Calculates each entry. Assumes inputs is cleaned paths.
func (a *Application) processWithCache(inputs []string) (*Entry, error) {
	a.Log.Printf("processWithCache(%d)", len(inputs))
	cache, err := a.LoadCache()
	if err != nil {
		return nil, err
	}
	defer cache.Close()

	entryRoot := &Entry{}
	// Throtttle after 128k entries per input.
	channels := make([]<-chan TreeItem, len(inputs))
	for i, input := range inputs {
		stat, err := os.Stat(input)
		if err != nil {
			// TODO(maruel): Leaks the channels and the go routines.
			return nil, err
		}
		if stat.IsDir() {
			channels[i] = EnumerateTree(input)
		} else {
			c := make(chan TreeItem, 1)
			channels[i] = c
			c <- TreeItem{FullPath: input, FileInfo: stat}
			close(c)
		}
	}
	count := 0
	size := int64(0)
	for _, c := range channels {
		for {
			if IsInterrupted() {
				break
			}
			item, ok := <-c
			if !ok {
				break
			}
			if item.Error != nil {
				// TODO(maruel): Leaks.
				return nil, item.Error
			}
			if item.FileInfo.IsDir() {
				continue
			}
			display := item.FullPath
			if len(display) > 50 {
				display = "..." + display[len(display)-50:]
			}
			fmt.Fprintf(a.Out, "%d files %1.1fmb Hashing %s...    \r", count, float64(size)/1024./1024., display)
			cacheKey, key := RecursePath(cache.Root(), entryRoot, item.FullPath)
			if err = UpdateFile(cacheKey, key, item); err != nil {
				return nil, err
			}
			count += 1
			size += item.FileInfo.Size()
		}
	}
	fmt.Fprintf(a.Out, "\n")
	if IsInterrupted() {
		return nil, errors.New("Ctrl-C'ed out")
	}
	return entryRoot, nil
}

type Stats struct {
	nbArchived int
	archived   int64
	nbSkipped  int
	skipped    int64
	stdout     io.Writer
}

func (s *Stats) recurseTree(itemPath string, entry *Entry, cas CasTable) error {
	if IsInterrupted() {
		return errors.New("Ctrl-C'ed out")
	}
	for relPath, file := range entry.Files {
		if err := s.recurseTree(path.Join(itemPath, relPath), file, cas); err != nil {
			return err
		}
	}
	if entry.Sha1 != "" {
		f, err := os.Open(itemPath)
		if err != nil {
			return nil
		}
		defer f.Close()
		err = cas.AddEntry(f, entry.Sha1)
		if os.IsExist(err) {
			s.nbSkipped += 1
			s.skipped += entry.Size
			err = nil
		} else if err == nil {
			s.nbArchived += 1
			s.archived += entry.Size
		}
		return err
	}
	if s.stdout != nil {
		fmt.Fprintf(s.stdout, "%d files %1.1fmb Archiving ...\r", s.nbArchived+s.nbSkipped, float64(s.archived+s.skipped)/1024./1024.)
	}
	return nil
}

func (a *Application) casArchive(entries *Entry, cas CasTable) (string, error) {
	a.Log.Printf("casArchive(%d entries)\n", entries.CountMembers())
	root := ""
	if filepath.Separator == '/' {
		root = "/"
	}
	stats := Stats{stdout: a.Out}
	err := stats.recurseTree(root, entries, cas)
	fmt.Fprintf(a.Out, "\n")
	// Serialize the entry file to archive it too.
	data, err := json.Marshal(&entries)
	if err != nil {
		return "", fmt.Errorf("Failed to marshall entry file: %s\n", err)
	}
	entrySha1, err := AddBytes(cas, data)
	if os.IsExist(err) {
		stats.nbSkipped += 1
		stats.skipped += int64(len(data))
	} else if err != nil {
		return "", fmt.Errorf("Failed to create %s: %s\n", entrySha1, err)
	} else {
		stats.nbArchived += 1
		stats.archived += int64(len(data))
	}
	a.Log.Printf(
		"Archived %d files (%d bytes) Skipped %d files, (%d bytes)\n",
		stats.nbArchived, stats.archived, stats.nbSkipped, stats.skipped)
	return entrySha1, nil
}

// Convert to absolute paths and evaluate environment variables.
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

func (a *Application) archiveMain(toArchiveArg string) error {
	cas, err := CommonFlag(true, true)
	if err != nil {
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
	a.Log.Printf("Found %d entries to backup in %s", len(inputs), toArchive)
	cleanupList(path.Dir(toArchive), inputs)
	entry, err := a.processWithCache(inputs)
	if err != nil {
		return err
	}

	// Now the archival part. Create the basic directory structure.
	nodes, err := LoadNodesTable(Root, cas, a.Log)
	if err != nil {
		return err
	}
	entrySha1, err := a.casArchive(entry, cas)
	if err != nil {
		return err
	}
	node := &Node{Entry: entrySha1, Comment: archiveComment}
	return nodes.AddEntry(node, path.Base(toArchive))
}

var cmdArchive = &Command{
	Run:       runArchive,
	UsageLine: "archive <.toArchive> -out <out>",
	ShortDesc: "archive files to a dumbcas archive",
	LongDesc:  "Archives files listed in <.toArchive> file to a directory in the DumbCas(tm) layout. Files listed may be in relative path or in absolute path and may contain environment variables.",
	Flag:      GetCommonFlags(),
}

// Flags.
var archiveComment string

func init() {
	cmdArchive.Flag.StringVar(&archiveComment, "comment", "", "Comment to embed in the file")
}

func runArchive(a *Application, cmd *Command, args []string) int {
	if len(args) != 1 {
		fmt.Fprintf(a.Err, "%s: Must only provide a .toArchive file.\n", a.Name)
		return 1
	}
	HandleCtrlC()
	if err := a.archiveMain(args[0]); err != nil {
		fmt.Fprintf(a.Err, "%s: %s\n", a.Name, err)
		return 1
	}
	return 0
}
