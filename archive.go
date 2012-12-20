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
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type EntryCache struct {
	Sha1       string                 `json:"h,omitempty"`
	Size       int64                  `json:"s,omitempty"`
	Timestamp  int64                  `json:"t,omitempty"` // In Unix() epoch.
	LastTested int64                  `json:"T,omitempty"` // Last time this file was tested for presence.
	Files      map[string]*EntryCache `json:"f,omitempty"`
}

// Prints the EntryCache in Yaml-inspired output.
func (e *EntryCache) Print(w io.Writer, indent string) {
	if e.Sha1 != "" {
		fmt.Fprintf(w, "%sSha1: %s\n", indent, e.Sha1)
		fmt.Fprintf(w, "%sSize: %d\n", indent, e.Size)
	}
	if e.Files != nil {
		for _, f := range e.SortedFiles() {
			fmt.Fprintf(w, "%s- '%s'\n", indent, f)
			e.Files[f].Print(w, indent+"  ")
		}
	}
}

func (e *EntryCache) SortedFiles() []string {
	if e.Files == nil {
		return []string{}
	}
	out := make([]string, 0, len(e.Files))
	for f, _ := range e.Files {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

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

func CountSizeCache(i *EntryCache) int {
	countI := 1
	if i.Files != nil {
		for _, v := range i.Files {
			countI += CountSizeCache(v)
		}
	}
	return countI
}

func CountSize(i *Entry) int {
	countI := 1
	if i.Files != nil {
		for _, v := range i.Files {
			countI += CountSize(v)
		}
	}
	return countI
}

func UpdateFile(cache *EntryCache, entry *Entry, item TreeItem) error {
	//log.Printf("UpdateFile(%s, %s)", item.FullPath)
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

func loadCache() (*os.File, *EntryCache, error) {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	cacheDir := path.Join(usr.HomeDir, ".dumbcas")
	if err := os.Mkdir(cacheDir, 0700); err != nil && !os.IsExist(err) {
		return nil, nil, fmt.Errorf("Failed to access %s: %s", cacheDir, err)
	}
	cache := &EntryCache{}
	cacheFile := path.Join(cacheDir, "cache.json")
	f, err := os.OpenFile(cacheFile, os.O_CREATE|os.O_RDWR, 0600)
	if f == nil {
		return nil, nil, fmt.Errorf("Failed to access %s: %s", cacheFile, err)
	}
	if data, err := ioutil.ReadAll(f); err == nil && len(data) != 0 {
		if err = json.Unmarshal(data, &cache); err != nil {
			// Ignore unmarshaling failure.
			cache = &EntryCache{}
		}
	}
	if _, err = f.Seek(0, 0); err != nil {
		return nil, nil, fmt.Errorf("Failed to seek %s: %s", cacheFile, err)
	}
	log.Printf("Loaded %d entries from the cache.", CountSizeCache(cache)-1)
	return f, cache, nil
}

func saveCache(f *os.File, cache *EntryCache) error {
	// TODO(maruel): When testing, the entries shouldn't be saved in the cache.
	// Trim anything > ~1yr old.
	one_year := time.Now().Unix() - (365 * 24 * 60 * 60)
	for relFile, file := range cache.Files {
		if file.LastTested < one_year {
			delete(cache.Files, relFile)
		}
	}
	log.Printf("Saving Cache: %d entries.", CountSizeCache(cache)-1)
	data, err := json.Marshal(&cache)
	if err != nil {
		return fmt.Errorf("Failed to marshall internal state: %s", err)
	}
	if err = f.Truncate(0); err != nil {
		return fmt.Errorf("Failed to truncate %s: %s", f.Name(), err)
	}
	if _, err = f.Write(data); err != nil {
		return fmt.Errorf("Failed to write %s: %s", f.Name(), err)
	}
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
func processWithCache(stdout io.Writer, inputs []string) (*Entry, error) {
	log.Printf("processWithCache(%d)", len(inputs))
	f, cache, err := loadCache()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entryRoot := &Entry{}
	// Throtttle after 128k entries per input.
	channels := make([]chan TreeItem, len(inputs))
	for i, input := range inputs {
		stat, err := os.Stat(input)
		if err != nil {
			return nil, err
		}
		if stat.IsDir() {
			channels[i] = make(chan TreeItem, 128*1024)
			go EnumerateTree(input, channels[i])
		} else {
			channels[i] = make(chan TreeItem, 2)
			channels[i] <- TreeItem{FullPath: input, FileInfo: stat}
			channels[i] <- TreeItem{}
		}
	}
	count := 0
	size := int64(0)
	for _, c := range channels {
		if Stop {
			break
		}
		for {
			if Stop {
				break
			}
			item := <-c
			if item.FullPath != "" {
				if item.FileInfo.IsDir() {
					continue
				}
				display := item.FullPath
				if len(display) > 50 {
					display = "..." + display[len(display)-50:]
				}
				fmt.Fprintf(stdout, "%d files %1.1fmb Hashing %s...    \r", count, float64(size)/1024./1024., display)
				cacheKey, key := RecursePath(cache, entryRoot, item.FullPath)
				if err = UpdateFile(cacheKey, key, item); err != nil {
					return nil, err
				}
				count += 1
				size += item.FileInfo.Size()
			} else if item.Error != nil {
				return nil, item.Error
			} else {
				break
			}
		}
	}
	fmt.Fprintf(stdout, "\n")
	// Save the cache right away in case archival fails.
	if err = saveCache(f, cache); err != nil {
		return nil, err
	}
	if Stop {
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
	if Stop {
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

func casArchive(stdout io.Writer, entries *Entry, cas CasTable) (string, error) {
	log.Printf("casArchive(%d entries)\n", CountSize(entries))
	root := ""
	if filepath.Separator == '/' {
		root = "/"
	}
	stats := Stats{stdout: stdout}
	err := stats.recurseTree(root, entries, cas)
	fmt.Fprintf(stdout, "\n")
	// Serialize the entry file to archive it too.
	data, err := json.Marshal(&entries)
	if err != nil {
		return "", fmt.Errorf("Failed to marshall entry file: %s\n", err)
	}
	entrySha1, err := cas.AddBytes(data)
	if os.IsExist(err) {
		stats.nbSkipped += 1
		stats.skipped += int64(len(data))
	} else if err != nil {
		return "", fmt.Errorf("Failed to create %s: %s\n", entrySha1, err)
	} else {
		stats.nbArchived += 1
		stats.archived += int64(len(data))
	}
	log.Printf(
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

func archiveMain(stdout io.Writer, toArchiveArg string) error {
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
	log.Printf("Found %d entries to backup in %s", len(inputs), toArchive)
	cleanupList(path.Dir(toArchive), inputs)
	entry, err := processWithCache(stdout, inputs)
	if err != nil {
		return err
	}

	// Now the archival part. Create the basic directory structure.
	nodesRoot := path.Join(Root, NodesName)
	if err := os.Mkdir(nodesRoot, 0750); err != nil && !os.IsExist(err) {
		return fmt.Errorf("Failed to create %s: %s\n", Root, err)
	}
	entrySha1, err := casArchive(stdout, entry, cas)
	if err != nil {
		return err
	}
	data, err := json.Marshal(&Node{Entry: entrySha1, Comment: archiveComment})
	if err != nil {
		return fmt.Errorf("Failed to marshall internal state: %s", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Failed to get the hostname: %s", err)
	}
	parts := strings.SplitN(hostname, ".", 2)
	hostname = parts[0]

	now := time.Now().UTC()
	// Create one directory store per month.
	monthName := now.Format("2006-01")
	monthDir := path.Join(nodesRoot, monthName)
	if err := os.MkdirAll(monthDir, 0750); err != nil && !os.IsExist(err) {
		return fmt.Errorf("Failed to create %s: %s\n", monthDir, err)
	}
	suffix := 0
	for {
		nodeName := hostname + "_" + now.Format("2006-01-02_15-04-05") + "_" + path.Base(toArchive)
		if suffix != 0 {
			nodeName += fmt.Sprintf("(%d)", suffix)
		}
		f, err := os.OpenFile(path.Join(monthDir, nodeName), os.O_WRONLY|os.O_EXCL|os.O_CREATE, 0640)
		if err != nil {
			// Try ad nauseam.
			suffix += 1
		} else {
			if _, err = f.Write(data); err != nil {
				return fmt.Errorf("Failed to write %s: %s", f.Name(), err)
			}
			log.Printf("Saved node: %s", path.Join(monthName, nodeName))
			break
		}
	}
	return nil
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
	if err := archiveMain(a.Out, args[0]); err != nil {
		fmt.Fprintf(a.Err, "%s: %s\n", a.Name, err)
		return 1
	}
	return 0
}
