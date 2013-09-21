// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fsnotify implements filesystem notification.
package fsnotify

import (
	"path/filepath"
	"strings"
	"sync"
	"time"
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
	triggers       Triggers             // event types to forward on
	patterns       []string             // file name patterns
	lastEventAt    map[string]time.Time // file name -> last ran for throttling
	lastEventMutex sync.Mutex
	steps          []stepFn // enabled pipeline steps to run
}

// stepFn filters an event, returning true to forward it on
type stepFn func(*pipeline, notifier) bool

// maximum steps in the pipeline
const maxSteps = 4

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

	// throttle setup
	if opt.Throttle {
		// TODO: ask adapter if it can handle throttling for us
		// TODO: leading/trailing and configurable latency
		p.lastEventAt = make(map[string]time.Time, 20)
		p.steps = append(p.steps, (*pipeline).throttleStep)
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

const throttleLatency = 1 * time.Second

// throttleStep
func (p *pipeline) throttleStep(ev notifier) bool {
	forward := true

	p.lastEventMutex.Lock()
	eventAt, ok := p.lastEventAt[ev.fileName()]
	if ok && time.Now().Sub(eventAt) <= throttleLatency {
		forward = false
	} else {
		p.lastEventAt[ev.fileName()] = time.Now()
	}
	p.lastEventMutex.Unlock()

	return forward
}
