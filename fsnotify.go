// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fsnotify implements file system notification.
package fsnotify

import "fmt"

const (
	FSN_CREATE = 1 << iota
	FSN_MODIFY
	FSN_DELETE
	FSN_RENAME
	FSN_CLOSE_WRITE

	FSN_ALL = FSN_MODIFY | FSN_DELETE | FSN_RENAME | FSN_CREATE | FSN_CLOSE_WRITE
)

// Purge events from interal chan to external chan if passes filter
func (w *Watcher) purgeEvents() {
	for ev := range w.internalEvent {
		sendEvent := false
		w.fsnmut.Lock()
		fsnFlags := w.fsnFlags[ev.Name]
		w.fsnmut.Unlock()

		if (fsnFlags&FSN_CREATE == FSN_CREATE) && ev.IsCreate() {
			sendEvent = true
		}

		if (fsnFlags&FSN_MODIFY == FSN_MODIFY) && ev.IsModify() {
			sendEvent = true
		}

		if (fsnFlags&FSN_DELETE == FSN_DELETE) && ev.IsDelete() {
			sendEvent = true
		}

		if (fsnFlags&FSN_RENAME == FSN_RENAME) && ev.IsRename() {
			sendEvent = true
		}

		// In some cases, we make sure a file change is finished before we read it.
		// So how do you do that? Try using IN_CLOSE_WRITE instead.
		//
		// https://stackoverflow.com/questions/32377517/inotify-event-in-modify-occurring-twice-for-tftp-put/32424150
		// Q: What is the difference between IN_MODIFY and IN_CLOSE_WRITE?
		// The IN_MODIFY event is emitted on a file content change (e.g. via the write() syscall)
		// while IN_CLOSE_WRITE occurs on closing the changed file.
		// It means each change operation causes one IN_MODIFY event (it may occur many times during manipulations with an open file)
		// whereas IN_CLOSE_WRITE is emitted only once (on closing the file).
		//
		// Q: Is it better to use IN_MODIFY or IN_CLOSE_WRITE?
		// It varies from case to case. Usually it is more suitable to use IN_CLOSE_WRITE
		// because if emitted the all changes on the appropriate file are safely written inside the file.
		// The IN_MODIFY event needn't mean that a file change is finished (data may remain in memory buffers in the application).
		// On the other hand, many logs and similar files must be monitored using IN_MODIFY -
		// in such cases where these files are permanently open and thus no IN_CLOSE_WRITE can be emitted.
		if (fsnFlags&FSN_CLOSE_WRITE == FSN_CLOSE_WRITE) && ev.IsCloseWrite() {
			sendEvent = true
		}

		if sendEvent {
			w.Event <- ev
		}

		// If there's no file, then no more events for user
		// BSD must keep watch for internal use (watches DELETEs to keep track
		// what files exist for create events)
		if ev.IsDelete() {
			w.fsnmut.Lock()
			delete(w.fsnFlags, ev.Name)
			w.fsnmut.Unlock()
		}
	}

	close(w.Event)
}

// Watch a given file path
func (w *Watcher) Watch(path string) error {
	return w.WatchFlags(path, FSN_ALL)
}

// Watch a given file path for a particular set of notifications (FSN_MODIFY etc.)
func (w *Watcher) WatchFlags(path string, flags uint32) error {
	w.fsnmut.Lock()
	w.fsnFlags[path] = flags
	w.fsnmut.Unlock()
	return w.watch(path)
}

// Remove a watch on a file
func (w *Watcher) RemoveWatch(path string) error {
	w.fsnmut.Lock()
	delete(w.fsnFlags, path)
	w.fsnmut.Unlock()
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

	if e.IsAttrib() {
		events += "|" + "ATTRIB"
	}

	if e.IsCloseWrite() {
		events += "|" + "CLOSE_WRITE"
	}

	if len(events) > 0 {
		events = events[1:]
	}

	return fmt.Sprintf("%q: %s", e.Name, events)
}
