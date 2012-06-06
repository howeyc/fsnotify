// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build freebsd openbsd netbsd darwin

//Package fsnotify implements filesystem notification.
package fsnotify

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
)

type FileEvent struct {
	mask   uint32 // Mask of events
	Name   string // File name (optional)
	create bool   // set by fsnotify package if found new file
}

// IsCreate reports whether the FileEvent was triggerd by a creation
func (e *FileEvent) IsCreate() bool { return e.create }

// IsDelete reports whether the FileEvent was triggerd by a delete
func (e *FileEvent) IsDelete() bool { return (e.mask & NOTE_DELETE) == NOTE_DELETE }

// IsModify reports whether the FileEvent was triggerd by a file modification
func (e *FileEvent) IsModify() bool {
	return ((e.mask&NOTE_WRITE) == NOTE_WRITE || (e.mask&NOTE_ATTRIB) == NOTE_ATTRIB)
}

// IsRename reports whether the FileEvent was triggerd by a change name
func (e *FileEvent) IsRename() bool { return (e.mask & NOTE_RENAME) == NOTE_RENAME }

type Watcher struct {
	kq       int                 // File descriptor (as returned by the kqueue() syscall)
	watches  map[string]int      // Map of watched file diescriptors (key: path)
	paths    map[int]string      // Map of watched paths (key: watch descriptor)
	finfo    map[int]os.FileInfo // Map of file information (isDir, isReg; key: watch descriptor)
	Error    chan error          // Errors are sent on this channel
	Event    chan *FileEvent     // Events are returned on this channel
	done     chan bool           // Channel for sending a "quit message" to the reader goroutine
	isClosed bool                // Set to true when Close() is first called
	kbuf     [1]syscall.Kevent_t // An event buffer for Add/Remove watch
}

// NewWatcher creates and returns a new kevent instance using kqueue(2)
func NewWatcher() (*Watcher, error) {
	fd, errno := syscall.Kqueue()
	if fd == -1 {
		return nil, os.NewSyscallError("kqueue", errno)
	}
	w := &Watcher{
		kq:      fd,
		watches: make(map[string]int),
		paths:   make(map[int]string),
		finfo:   make(map[int]os.FileInfo),
		Event:   make(chan *FileEvent),
		Error:   make(chan error),
		done:    make(chan bool, 1),
	}

	go w.readEvents()
	return w, nil
}

// Close closes a kevent watcher instance
// It sends a message to the reader goroutine to quit and removes all watches
// associated with the kevent instance
func (w *Watcher) Close() error {
	if w.isClosed {
		return nil
	}
	w.isClosed = true

	// Send "quit" message to the reader goroutine
	w.done <- true
	for path := range w.watches {
		w.RemoveWatch(path)
	}

	return nil
}

// AddWatch adds path to the watched file set.
// The flags are interpreted as described in kevent(2).
func (w *Watcher) addWatch(path string, flags uint32) error {
	if w.isClosed {
		return errors.New("kevent instance already closed")
	}

	watchEntry := &w.kbuf[0]
	watchEntry.Fflags = flags

	watchfd, found := w.watches[path]
	if !found {
		fi, errstat := os.Lstat(path)
		if errstat != nil {
			return errstat
		}

		// Follow Symlinks
		// Unfortunately, Linux can add bogus symlinks to watch list without
		// issue, and Windows can't do symlinks period (AFAIK). To  maintain
		// consistency, we will act like everything is fine. There will simply
		// be no file events for broken symlinks.
		// Hence the returns of nil on errors.
		if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
			path, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil
			}

			fi, errstat = os.Lstat(path)
			if errstat != nil {
				return nil
			}
		}

		fd, errno := syscall.Open(path, syscall.O_NONBLOCK|syscall.O_RDONLY, 0700)
		if fd == -1 {
			return errno
		}
		watchfd = fd

		w.watches[path] = watchfd
		w.paths[watchfd] = path

		w.finfo[watchfd] = fi
		if fi.IsDir() {
			errdir := w.watchDirectoryFiles(path)
			if errdir != nil {
				return errdir
			}
		}
	}
	syscall.SetKevent(watchEntry, watchfd, syscall.EVFILT_VNODE, syscall.EV_ADD|syscall.EV_CLEAR)

	wd, errno := syscall.Kevent(w.kq, w.kbuf[:], nil, nil)
	if wd == -1 {
		return errno
	} else if (watchEntry.Flags & syscall.EV_ERROR) == syscall.EV_ERROR {
		return errors.New("kevent add error")
	}

	return nil
}

// Watch adds path to the watched file set, watching all events.
func (w *Watcher) Watch(path string) error {
	return w.addWatch(path, NOTE_ALLEVENTS)
}

// RemoveWatch removes path from the watched file set.
func (w *Watcher) RemoveWatch(path string) error {
	watchfd, ok := w.watches[path]
	if !ok {
		return errors.New(fmt.Sprintf("can't remove non-existent kevent watch for: %s", path))
	}
	syscall.Close(watchfd)
	watchEntry := &w.kbuf[0]
	syscall.SetKevent(watchEntry, w.watches[path], syscall.EVFILT_VNODE, syscall.EV_DELETE)
	success, errno := syscall.Kevent(w.kq, w.kbuf[:], nil, nil)
	if success == -1 {
		return os.NewSyscallError("kevent_rm_watch", errno)
	} else if (watchEntry.Flags & syscall.EV_ERROR) == syscall.EV_ERROR {
		return errors.New("kevent rm error")
	}
	delete(w.watches, path)
	return nil
}

// readEvents reads from the kqueue file descriptor, converts the
// received events into Event objects and sends them via the Event channel
func (w *Watcher) readEvents() {
	var (
		eventbuf [10]syscall.Kevent_t // Event buffer
		events   []syscall.Kevent_t   // Received events
		twait    *syscall.Timespec    // Time to block waiting for events
		n        int                  // Number of events returned from kevent
		errno    error                // Syscall errno
	)
	events = eventbuf[0:0]
	twait = new(syscall.Timespec)
	*twait = syscall.NsecToTimespec(keventWaitTime)

	for {
		// See if there is a message on the "done" channel
		var done bool
		select {
		case done = <-w.done:
		default:
		}

		// If "done" message is received
		if done {
			errno := syscall.Close(w.kq)
			if errno != nil {
				w.Error <- os.NewSyscallError("close", errno)
			}
			close(w.Event)
			close(w.Error)
			return
		}

		// Get new events
		if len(events) == 0 {
			n, errno = syscall.Kevent(w.kq, nil, eventbuf[:], twait)

			// EINTR is okay, basically the syscall was interrupted before 
			// timeout expired.
			if errno != nil && errno != syscall.EINTR {
				w.Error <- os.NewSyscallError("kevent", errno)
				continue
			}

			// Received some events
			if n > 0 {
				events = eventbuf[0:n]
			}
		}

		// Flush the events we recieved to the events channel
		for len(events) > 0 {
			fileEvent := new(FileEvent)
			watchEvent := &events[0]
			fileEvent.mask = uint32(watchEvent.Fflags)
			fileEvent.Name = w.paths[int(watchEvent.Ident)]

			fileInfo := w.finfo[int(watchEvent.Ident)]
			if fileInfo.IsDir() && fileEvent.IsModify() {
				w.sendDirectoryChangeEvents(fileEvent.Name)
			} else {
				// Send the event on the events channel
				w.Event <- fileEvent
			}

			// Move to next event
			events = events[1:]
		}
	}
}

func (w *Watcher) watchDirectoryFiles(dirPath string) error {
	// Get all files
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return err
	}

	// Search for new files
	for _, fileInfo := range files {
		if fileInfo.IsDir() == false {
			filePath := filepath.Join(dirPath, fileInfo.Name())
			// Watch file to mimic linux fsnotify
			e := w.addWatch(filePath, NOTE_DELETE|NOTE_WRITE|NOTE_RENAME)
			if e != nil {
				return e
			}
		}
	}

	return nil
}

// sendDirectoryEvents searches the directory for newly created files
// and sends them over the event channel. This functionality is to have
// the BSD version of fsnotify mach linux fsnotify which provides a 
// create event for files created in a watched directory.
func (w *Watcher) sendDirectoryChangeEvents(dirPath string) {
	// Get all files
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		w.Error <- err
	}

	// Search for new files
	for _, fileInfo := range files {
		if fileInfo.IsDir() == false {
			filePath := filepath.Join(dirPath, fileInfo.Name())
			if w.watches[filePath] == 0 {
				// Send create event
				fileEvent := new(FileEvent)
				fileEvent.Name = filePath
				fileEvent.create = true
				w.Event <- fileEvent
			}
		}
	}
	w.watchDirectoryFiles(dirPath)
}

const (
	// Flags (from <sys/event.h>)
	NOTE_DELETE = 0x0001 /* vnode was removed */
	NOTE_WRITE  = 0x0002 /* data contents changed */
	NOTE_EXTEND = 0x0004 /* size increased */
	NOTE_ATTRIB = 0x0008 /* attributes changed */
	NOTE_LINK   = 0x0010 /* link count changed */
	NOTE_RENAME = 0x0020 /* vnode was renamed */
	NOTE_REVOKE = 0x0040 /* vnode access was revoked */

	// Watch all events
	NOTE_ALLEVENTS = NOTE_DELETE | NOTE_WRITE | NOTE_ATTRIB | NOTE_RENAME

	// Block for 100 ms on each call to kevent
	keventWaitTime = 100e6
)
