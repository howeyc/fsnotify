// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fsnotify implements filesystem notification.
package fsnotify

import (
	"path/filepath"
	"strings"
)

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
	patterns []string
	steps    []stepFn // enabled pipeline steps to run
}

// stepFn filters an event, returning true to forward it on
type stepFn func(*pipeline, notifier) bool

// maximum steps in the pipeline
const maxSteps = 1

// newPipeline creates a pipeline and enables the steps
func newPipeline(opt *Options) pipeline {
	p := pipeline{steps: make([]stepFn, 0, maxSteps)}

	// setup pipeline steps, order matters

	// TODO: Verbose option to setup loggingStep

	// hidden setup
	if !opt.Hidden {
		p.steps = append(p.steps, (*pipeline).hiddenStep)
	}

	// TODO: Recursive option may setup autoWatchStep (consult adapter)
	// watch created directories unless they are hidden but even if ignoring Create Trigger

	// triggers setup
	if opt.Triggers != allEvents && opt.Triggers != 0 {
		p.triggers = opt.Triggers
		p.steps = append(p.steps, (*pipeline).triggerStep)
	}

	// pattern setup
	if opt.Pattern != "" {
		p.patterns = strings.Split(opt.Pattern, ",")
		p.steps = append(p.steps, (*pipeline).patternStep)
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

// hiddenStep discards events for hidden files (.DS_Store, .subl26d.tmp) and directories (.git, .hg, .bzr)
func (p *pipeline) hiddenStep(ev notifier) bool {
	return !isHidden(ev.fileName())
}

func isHidden(name string) bool {
	// TODO: what about hidden on Windows?
	return strings.HasPrefix(name, ".") && name != "." && name != ".."
}

// triggerStep discards any combination of create, modify, delete, or rename events
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

// patternStep discards events that don't match one of the shell file name patterns
func (p *pipeline) patternStep(ev notifier) bool {
	for _, pattern := range p.patterns {
		matched, err := filepath.Match(pattern, ev.fileName())
		// treat ErrBadPattern as a non-match:
		if err == nil && matched {
			return true
		}
	}
	return false
}
