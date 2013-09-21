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
	fileName() string // or should this be Path()
}

/*
  A pipeline to process events
*/
type pipeline struct {
	triggers Triggers
	steps    []stepFn // enabled pipeline steps to run
}

// stepFn filters an event, returning true to forward it on
type stepFn func(*pipeline, notifier) bool

// maximum steps in the pipeline
const maxSteps = 1

// newPipeline creates a pipeline and enables the steps
func newPipeline(opt options) pipeline {
	p := pipeline{steps: make([]stepFn, 0, maxSteps)}

	// triggers setup
	if opt.triggers != allEvents && opt.triggers != 0 {
		p.triggers = opt.triggers
		p.steps = append(p.steps, (*pipeline).triggerStep)
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
func (p *pipeline) triggerStep(ev notifier) bool {
	if (p.triggers&Create == Create) && ev.IsCreate() {
		return true
	}

	if (p.triggers&Modify == Modify) && ev.IsModify() {
		return true
	}

	if (p.triggers&Delete == Delete) && ev.IsDelete() {
		return true
	}

	if (p.triggers&Rename == Rename) && ev.IsRename() {
		return true
	}

	return false
}
