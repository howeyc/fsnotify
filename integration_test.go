// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fsnotify_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/howeyc/fsnotify"
)

var r *rand.Rand

func init() {
	r = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func testTempDir() string {
	osTempDir := os.TempDir()
	randDir := fmt.Sprintf("%d", r.Int())
	return filepath.Join(osTempDir, randDir)
}

func TestFsnotifyRecursive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create an fsnotify watcher instance and initialize it
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() failed: %s", err)
	}

	testDir := testTempDir()

	// Create directory to watch
	if err := os.Mkdir(testDir, 0777); err != nil {
		t.Fatalf("failed to create test directory: %s", err)
	}
	defer os.RemoveAll(testDir)

	// Add a watch for testDir
	err = watcher.WatchPath(testDir, &fsnotify.Options{Recursive: true, Hidden: true})
	if err != nil {
		t.Fatalf("watcher.Watch() failed: %s", err)
	}

	// Receive errors on the error channel on a separate goroutine
	go func() {
		for err := range watcher.Error {
			t.Fatalf("error received: %s", err)
		}
	}()

	createEvents := 0
	expectCreateEvents := 2

	done := make(chan bool)
	go func() {
		for ev := range watcher.Event {
			// println(ev.String())
			if ev.IsCreate() {
				createEvents++
				if createEvents == expectCreateEvents {
					watcher.Close()
				}
			}
		}
		done <- true
	}()

	subDir := filepath.Join(testDir, "subdir")
	if err := os.Mkdir(subDir, 0777); err != nil {
		t.Fatalf("failed to create subdir in test directory: %s", err)
	}

	// Create a file in the subDir (subDir should be autoWatched)
	testFile := filepath.Join(subDir, "TestFsnotifyRecursive.testfile")

	var f *os.File
	f, err = os.OpenFile(testFile, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Fatalf("creating test file failed: %s", err)
	}
	f.Sync()
	f.Close()

	select {
	case <-done:
		t.Log("event channel closed")
	case <-time.After(2 * time.Second):
	}

	if createEvents != expectCreateEvents {
		t.Fatalf("expected %d create events, but received %d", expectCreateEvents, createEvents)
	}
}
