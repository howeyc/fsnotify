// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fsnotify

/*
	A cross-platform implementation of recursive watching

	This should be replaced with OS-specific recursive watching where available
*/

import (
	"errors"
	"os"
	"path/filepath"
)

// Watch a given file path recursively
func (w *Watcher) watchRecursively(path string, pipeline pipeline) error {
	folders := subdirectories(path, pipeline.hidden)
	if len(folders) == 0 {
		return errors.New("No folders to watch.")
	}

	for _, folder := range folders {
		err := w.watch(folder, pipeline)
		if err != nil {
			// TODO: remove watches that were already added
			return err
		}
	}

	return nil
}

// TODO: removeWatchRecurisvely

// subdirectories lists the directories below a the path, including the path passed in
func subdirectories(path string, includeHidden bool) (paths []string) {
	filepath.Walk(path, func(newPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			name := info.Name()
			// skip directories that begin with a dot (.git, .hg, .bzr)
			if !includeHidden && isHidden(name) {
				return filepath.SkipDir
			} else {
				paths = append(paths, newPath)
			}
		}
		return nil
	})
	return paths
}
