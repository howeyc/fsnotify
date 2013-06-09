// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package fsnotify

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	// Options for AddWatch
	sys_FS_ONESHOT = 0x80000000
	sys_FS_ONLYDIR = 0x1000000

	// Events
	sys_FS_ACCESS      = 0x1
	sys_FS_ALL_EVENTS  = 0xfff
	sys_FS_ATTRIB      = 0x4
	sys_FS_CLOSE       = 0x18
	sys_FS_CREATE      = 0x100
	sys_FS_DELETE      = 0x200
	sys_FS_DELETE_SELF = 0x400
	sys_FS_MODIFY      = 0x2
	sys_FS_MOVE        = 0xc0
	sys_FS_MOVED_FROM  = 0x40
	sys_FS_MOVED_TO    = 0x80
	sys_FS_MOVE_SELF   = 0x800

	// Special events
	sys_FS_IGNORED    = 0x8000
	sys_FS_Q_OVERFLOW = 0x4000
)

// Event is the type of the notification messages
// received on the watcher's Event channel.
type FileEvent struct {
	mask   uint32 // Mask of events
	cookie uint32 // Unique cookie associating related events (for rename)
	Name   string // File name (optional)
}

// IsCreate reports whether the FileEvent was triggerd by a creation
func (e *FileEvent) IsCreate() bool { return (e.mask & sys_FS_CREATE) == sys_FS_CREATE }

// IsDelete reports whether the FileEvent was triggerd by a delete
func (e *FileEvent) IsDelete() bool {
	return ((e.mask&sys_FS_DELETE) == sys_FS_DELETE || (e.mask&sys_FS_DELETE_SELF) == sys_FS_DELETE_SELF)
}

// IsModify reports whether the FileEvent was triggerd by a file modification or attribute change
func (e *FileEvent) IsModify() bool {
	return ((e.mask&sys_FS_MODIFY) == sys_FS_MODIFY || (e.mask&sys_FS_ATTRIB) == sys_FS_ATTRIB)
}

// IsRename reports whether the FileEvent was triggerd by a change name
func (e *FileEvent) IsRename() bool {
	return ((e.mask&sys_FS_MOVE) == sys_FS_MOVE || (e.mask&sys_FS_MOVE_SELF) == sys_FS_MOVE_SELF || (e.mask&sys_FS_MOVED_FROM) == sys_FS_MOVED_FROM || (e.mask&sys_FS_MOVED_TO) == sys_FS_MOVED_TO)
}

// A Watcher waits for and receives event notifications
// for a specific set of files and directories.
type Watcher struct {
	mu            sync.Mutex               // Map access
	dirWatches    map[string]chan struct{} // Map of directories to channel that closes watch goroutine
	fsnFlags      map[string]uint32        // Map of watched files to flags used for filter
	fsnmut        sync.Mutex               // Protects access to fsnFlags.
	internalEvent chan *FileEvent          // Events are queued on this channel
	Event         chan *FileEvent          // Events are returned on this channel
	Error         chan error               // Errors are sent on this channel
	isClosed      bool
}

// NewWatcher creates and returns a Watcher.
func NewWatcher() (*Watcher, error) {
	w := &Watcher{
		dirWatches:    make(map[string]chan struct{}),
		fsnFlags:      make(map[string]uint32),
		Event:         make(chan *FileEvent),
		internalEvent: make(chan *FileEvent),
		Error:         make(chan error),
	}

	go w.purgeEvents()
	return w, nil
}

// Close closes a Watcher.
// It sends a message to the reader goroutine to quit and removes all watches
// associated with the watcher.
func (w *Watcher) Close() error {
	if w.isClosed {
		return nil
	}
	w.isClosed = true

	// Quit each directory watcher
	for _, watchChan := range w.dirWatches {
		close(watchChan)
	}
	close(w.Event)
	close(w.Error)
	return nil
}

// watch adds path to the watched file set.
func (w *Watcher) watch(path string) error {
	if w.isClosed {
		return errors.New("watcher is closed")
	}

	dir := filepath.Dir(path)
	w.mu.Lock()
	if _, watchExists := w.dirWatches[dir]; !watchExists {
		if handle, err := syscall.CreateFile(syscall.StringToUTF16Ptr(path),
			syscall.FILE_LIST_DIRECTORY,
			syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
			nil, syscall.OPEN_EXISTING,
			syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OVERLAPPED, 0); err != nil {
			return os.NewSyscallError("CreateFile", err)
		} else {
			ch := make(chan struct{})
			w.dirWatches[dir] = ch
			w.watchDirectory(handle, path, ch)
			time.Sleep(50 * time.Millisecond)
		}
	}
	w.mu.Unlock()
	return nil
}

// RemoveWatch removes path from the watched file set.
func (w *Watcher) removeWatch(path string) error {
	w.mu.Lock()
	if watchChan, watchExists := w.dirWatches[path]; watchExists {
		close(watchChan)
		delete(w.dirWatches, path)
	} else if _, watchDir := w.dirWatches[filepath.Dir(path)]; !watchDir {
		return errors.New("file was not being watched")
	}
	return nil
}

// Must run within the I/O thread.
func (w *Watcher) watchDirectory(handle syscall.Handle, path string, doneChan <-chan struct{}) {
	fnEvents := make(chan *FileEvent)
	quit := make(chan bool, 1)
	dir := filepath.Base(path)

	go func() {
		var buf [4096]byte
		var bytesReturned uint32
		for {
			syscall.ReadDirectoryChanges(handle, &buf[0],
				uint32(unsafe.Sizeof(buf)), false, sys_FS_ALL_EVENTS, &bytesReturned, nil, 0)
			if bytesReturned > 0 {
				// Point "raw" to the event in the buffer
				var offset uint32
				for {
					raw := (*syscall.FileNotifyInformation)(unsafe.Pointer(&buf[offset]))
					ebuf := (*[syscall.MAX_PATH]uint16)(unsafe.Pointer(&raw.FileName))
					name := syscall.UTF16ToString(ebuf[:raw.FileNameLength/2])
					fullname := dir + "\\" + name

					var mask uint32
					switch raw.Action {
					case syscall.FILE_ACTION_ADDED:
						mask = sys_FS_CREATE
					case syscall.FILE_ACTION_REMOVED:
						mask = sys_FS_DELETE_SELF
					case syscall.FILE_ACTION_MODIFIED:
						mask = sys_FS_MODIFY
					case syscall.FILE_ACTION_RENAMED_OLD_NAME:
						mask = sys_FS_MOVE_SELF
					case syscall.FILE_ACTION_RENAMED_NEW_NAME:
						mask = sys_FS_MOVE_SELF
					}

					fnEvents <- &FileEvent{Name: fullname, mask: mask}

					// Move to the next event in the buffer
					if raw.NextEntryOffset == 0 {
						break
					}
					offset += raw.NextEntryOffset
				}
			}
			select {
			case <-quit:
				return
			default:
			}
		}
	}()

	go func() {
		for {
			select {
			case ev := <-fnEvents:
				w.fsnmut.Lock()
				if fsnFlags, exists := w.fsnFlags[filepath.Dir(ev.Name)]; exists {
					w.fsnFlags[ev.Name] = fsnFlags
				} else {
					w.fsnFlags[ev.Name] = FSN_ALL
				}
				w.fsnmut.Unlock()
				w.internalEvent <- ev
			case _, open := <-doneChan:
				if !open {
					quit <- true
					syscall.CloseHandle(handle)
					return
				}
			}
		}
	}()
}
