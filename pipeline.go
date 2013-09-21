// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fsnotify implements filesystem notification.
package fsnotify

//  notifier is the interface for notifications/events
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
	fsnFlags uint32   // flags used for triggers() filter
	steps    []stepFn // enabled pipeline steps to run
}

// stepFn filters an event, returning true to forward it on
type stepFn func(*pipeline, notifier) bool

// maximum steps in the pipeline
const maxSteps = 1

// newPipeline creates a pipeline and enables the steps
// this function signature is gonna change!
func newPipeline(flags uint32) pipeline {
	p := pipeline{steps: make([]stepFn, 0, maxSteps)}

	// triggers step
	if flags != FSN_ALL {
		p.fsnFlags = flags
		p.steps = append(p.steps, (*pipeline).triggers)
	}

	return p
}

// processes an event and returns true if it should be forwarded
func (p *pipeline) processEvent(event notifier) bool {
	forward := true
	for _, process := range p.steps {
		if !process(p, event) {
			forward = false
			// TODO: may want to abort running the remaining pipeline steps
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
