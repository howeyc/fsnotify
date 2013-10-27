// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fsnotify implements file system notifications.
package fsnotify

import "fmt"

// Options for watching paths
type Options struct {
	Verbose   bool     // log events, helpful for debugging
	Hidden    bool     // include hidden files (.DS_Store) and directories (.git, .hg)
	Triggers  Triggers // Create | Modify | Delete | Rename events (default: all)
	Pattern   string   // comma separated list of shell file name patterns (see filepath.Match)
	Throttle  bool     // events on a file are discarded for the next second
	Recursive bool     // watch all subdirectories of the specified path
}

// Trigger types to watch for
type Triggers uint32

const (
	Create Triggers = 1 << iota
	Modify
	Delete
	Rename

	allTriggers Triggers = Modify | Delete | Rename | Create
)

const (
	// DEPRECATION(-): please use Triggers
	FSN_CREATE = 1
	FSN_MODIFY = 2
	FSN_DELETE = 4
	FSN_RENAME = 8

	FSN_ALL = FSN_MODIFY | FSN_DELETE | FSN_RENAME | FSN_CREATE
)

// Forward events from internal channel to external channel if passes filter
func (w *Watcher) forwardEvents() {
	for ev := range w.internalEvent {
		w.pipelinesmut.Lock()
		pipeline := w.pipelines[ev.Path()]
		w.pipelinesmut.Unlock()

		forward := pipeline.processEvent(ev)
		if forward {
			w.Event <- ev
		}

		// If there's no file, then no more events for user
		// BSD must keep watch for internal use (watches DELETEs to keep track
		// what files exist for create events)
		if ev.IsDelete() {
			w.pipelinesmut.Lock()
			delete(w.pipelines, ev.Path())
			w.pipelinesmut.Unlock()
		}
	}

	close(w.Event)
}

// WatchPath watches a given file path with a particular set of options
func (w *Watcher) WatchPath(path string, options *Options) (err error) {
	// TODO: check adapter capabilities
	if options.Recursive {
		return w.watchRecursively(path, options)
	}
	return w.watch(path, options)
}

// DEPRECATION(-): please use WatchPath()
// Watch a given file path
func (w *Watcher) Watch(path string) error {
	return w.WatchPath(path, &Options{Triggers: allTriggers, Hidden: true})
}

// DEPRECATION(-): please use WatchPath()
// Watch a given file path for a particular set of notifications (FSN_MODIFY etc.)
func (w *Watcher) WatchFlags(path string, flags Triggers) error {
	return w.WatchPath(path, &Options{Triggers: flags, Hidden: true})
}

// Remove a watch on a file
func (w *Watcher) RemoveWatch(path string) error {
	w.pipelinesmut.Lock()
	delete(w.pipelines, path)
	w.pipelinesmut.Unlock()
	return w.removeWatch(path)
}

// String formats the event e in the form
// "filename: DELETE|MODIFY|..."
func (e *FileEvent) String() string {
	var events string = ""

	if e.IsCreate() {
		events += "|" + "CREATE"
	}

	if e.IsDelete() {
		events += "|" + "DELETE"
	}

	if e.IsModify() {
		events += "|" + "MODIFY"
	}

	if e.IsRename() {
		events += "|" + "RENAME"
	}

	if len(events) > 0 {
		events = events[1:]
	}

	return fmt.Sprintf("%q: %s", e.Path(), events)
}
