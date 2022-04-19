package keystore

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ChainAAS/gendchain/log"
)

// fileCache is a cache of files seen during scan of keystore.
type fileCache struct {
	all     map[string]struct{} // Set of all files from the keystore folder
	lastMod time.Time           // Last time instance when a file was modified
	mu      sync.RWMutex
}

// scan performs a new scan on the given directory, compares against the already
// cached filenames, and returns file sets: creates, deletes, updates.
func (fc *fileCache) scan(keyDir string) (map[string]struct{}, map[string]struct{}, map[string]struct{}, error) {
	t0 := time.Now()

	// List all the failes from the keystore folder
	files, err := ioutil.ReadDir(keyDir)
	if err != nil {
		return nil, nil, nil, err
	}
	t1 := time.Now()

	fc.mu.Lock()
	defer fc.mu.Unlock()

	// Iterate all the files and gather their metadata
	all := make(map[string]struct{})
	mods := make(map[string]struct{})

	var newLastMod time.Time
	for _, fi := range files {
		// Skip any non-key files from the folder
		path := filepath.Join(keyDir, fi.Name())
		if skipKeyFile(fi) {
			log.Trace("Ignoring file on account scan", "path", path)
			continue
		}
		// Gather the set of all and fresly modified files
		all[path] = struct{}{}

		modified := fi.ModTime()
		if modified.After(fc.lastMod) {
			mods[path] = struct{}{}
		}
		if modified.After(newLastMod) {
			newLastMod = modified
		}
	}
	t2 := time.Now()

	// Update the tracked files and return the three sets.

	// Deletes = previous - current
	deletes := make(map[string]struct{})
	for p := range fc.all {
		if _, ok := all[p]; !ok {
			deletes[p] = struct{}{}
		}
	}

	// Creates = current - previous
	creates := make(map[string]struct{})
	for p := range all {
		if _, ok := fc.all[p]; !ok {
			creates[p] = struct{}{}
		}
	}

	// Updates = modified - creates
	updates := make(map[string]struct{})
	for p := range mods {
		if _, ok := creates[p]; !ok {
			updates[p] = struct{}{}
		}
	}

	fc.all, fc.lastMod = all, newLastMod
	t3 := time.Now()

	// Report on the scanning stats and return
	log.Debug("FS scan times", "list", t1.Sub(t0), "set", t2.Sub(t1), "diff", t3.Sub(t2))
	return creates, deletes, updates, nil
}

// skipKeyFile ignores editor backups, hidden files and folders/symlinks.
func skipKeyFile(fi os.FileInfo) bool {
	// Skip editor backups and UNIX-style hidden files.
	if strings.HasSuffix(fi.Name(), "~") || strings.HasPrefix(fi.Name(), ".") {
		return true
	}
	// Skip misc special files, directories (yes, symlinks too).
	if fi.IsDir() || fi.Mode()&os.ModeType != 0 {
		return true
	}
	return false
}
