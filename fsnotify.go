// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fsnotify implements filesystem notification.
package fsnotify

import "fmt"

const (
	FSN_CREATE = 1
	FSN_MODIFY = 2
	FSN_DELETE = 4
	FSN_RENAME = 8

	FSN_ALL = FSN_MODIFY | FSN_DELETE | FSN_RENAME | FSN_CREATE
)

// Forward events from interal chan to external chan if passes filter
func (w *Watcher) forwardEvents() {
	for ev := range w.internalEvent {
		w.pipelinesmut.Lock()
		pipeline := w.pipelines[ev.fileName()]
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
			delete(w.pipelines, ev.fileName())
			w.pipelinesmut.Unlock()
		}
	}

	close(w.Event)
}

// Watch a given file path
func (w *Watcher) Watch(path string) error {
	w.pipelinesmut.Lock()
	w.pipelines[path] = pipeline{fsnFlags: FSN_ALL}
	w.pipelinesmut.Unlock()
	return w.watch(path)
}

// Watch a given file path for a particular set of notifications (FSN_MODIFY etc.)
func (w *Watcher) WatchFlags(path string, flags uint32) error {
	w.pipelinesmut.Lock()
	w.pipelines[path] = pipeline{fsnFlags: flags}
	w.pipelinesmut.Unlock()
	return w.watch(path)
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

	return fmt.Sprintf("%q: %s", e.fileName(), events)
}
