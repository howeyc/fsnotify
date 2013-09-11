// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fsnotify implements filesystem notification.
package fsnotify

type notifier interface {
	IsCreate() bool
	IsDelete() bool
	IsModify() bool
	IsRename() bool
	fileName() string
}

/*
  A pipeline to process events
*/
type pipeline struct {
	fsnFlags uint32 // flags used for triggers() filter
}

// step filters an event, returning true to forward it on
type step func(*pipeline, notifier) bool

// the full pipeline, order is important
var allSteps = []step{
	(*pipeline).triggers,
}

// processes an event and returns true if it should be forwarded
func (p *pipeline) processEvent(event notifier) bool {
	forward := true
	for _, process := range allSteps {
		if !process(p, event) {
			forward = false
		}
	}
	return forward
}

// triggers discards any combination of create, modify, delete, or rename events
func (p *pipeline) triggers(ev notifier) bool {
	if (p.fsnFlags&FSN_CREATE == FSN_CREATE) && ev.IsCreate() {
		return true
	}

	if (p.fsnFlags&FSN_MODIFY == FSN_MODIFY) && ev.IsModify() {
		return true
	}

	if (p.fsnFlags&FSN_DELETE == FSN_DELETE) && ev.IsDelete() {
		return true
	}

	if (p.fsnFlags&FSN_RENAME == FSN_RENAME) && ev.IsRename() {
		return true
	}

	return false
}
