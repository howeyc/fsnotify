# File system notifications for Go

[![Build Status](https://goci.herokuapp.com/project/image/github.com/howeyc/fsnotify)](http://goci.me/project/github.com/howeyc/fsnotify)

[GoDoc](http://go.pkgdoc.org/github.com/howeyc/fsnotify)

Cross platform, works on:
* Windows
* Linux
* BSD
* OSX

Example:
```go
package main

import (
	"log"

	"github.com/howeyc/fsnotify"
)

func main() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	done := make(chan bool)

	// Process events
	go func() {
		for {
			select {
			case ev := <-watcher.Event:
				log.Println("event:", ev)
			case err := <-watcher.Error:
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Watch("testDir")
	if err != nil {
		log.Fatal(err)
	}

	<-done

	/* ... do stuff ... */
	watcher.Close()
}
```

For each event:
* Name
* IsCreate()
* IsDelete()
* IsModify()
* IsRename()

Notes:
* When a file is renamed to another directory is it still being watched?
    * No (it shouldn't be, unless you are watching where it was moved to).
* When I watch a directory, are all subdirectories watched as well?
    * No, you must add watches for any directory you want to watch.
* Do I have to watch the Error and Event channels in a separate goroutine?
    * As of now, yes. Looking into making this single-thread friendly.
* There are OS-specific limits as to how many watches can be created:
    * Linux: /proc/sys/fs/inotify/max_user_watches contains the limit,
    reaching this limit results in a "no space left on device" error.
    * BSD / OSX: sysctl variables "kern.maxfiles" and "kern.maxfilesperproc",
    reaching these limits results in a "too many open files" error.
