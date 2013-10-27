// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fsnotify

import (
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//  Event represents a single file system event
type Event interface {
	IsCreate() bool
	IsDelete() bool
	IsModify() bool
	IsRename() bool
	Path() string // relative path to the file
}

/*
  A pipeline to process events
*/
type pipeline struct {
	verbose        bool
	triggers       Triggers             // event types to forward on
	patterns       []string             // file name patterns
	lastEventAt    map[string]time.Time // file name -> last ran for throttling
	lastEventMutex sync.Mutex
	steps          []stepFn // enabled pipeline steps to run
}

// stepFn filters an event, returning true to forward it on
type stepFn func(*pipeline, Event) bool

// maximum steps in the pipeline
const maxSteps = 4

// newPipeline creates a pipeline and enables the steps
func newPipeline(opt *Options) pipeline {
	p := pipeline{steps: make([]stepFn, 0, maxSteps)}

	// setup pipeline steps, order matters

	// logging setup
	if opt.Verbose {
		p.verbose = true
		p.steps = append(p.steps, (*pipeline).verboseStep)
	}

	// hidden setup
	if !opt.Hidden {
		p.steps = append(p.steps, (*pipeline).hiddenStep)
	}

	// TODO: Recursive option may setup autoWatchStep (consult adapter)
	// watch created directories unless they are hidden but even if ignoring Create Trigger

	// triggers setup
	if opt.Triggers != allTriggers && opt.Triggers != 0 {
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
func (p *pipeline) processEvent(event Event) bool {
	for _, process := range p.steps {
		if !process(p, event) {
			// early abort, don't run other pipeline steps
			return false
		}
	}
	return true // forward event
}

// verboseStep logs events
func (p *pipeline) verboseStep(event Event) bool {
	log.Printf("new event %v", event)
	return true
}

// hiddenStep discards events for hidden files (.DS_Store, .subl26d.tmp) and directories (.git, .hg, .bzr)
func (p *pipeline) hiddenStep(event Event) bool {
	forward := !isHidden(event.Path())
	if p.verbose && !forward {
		log.Printf("hidden cancels %v", event)
	}
	return forward
}

// triggerStep discards any combination of create, modify, delete, or rename events
func (p *pipeline) triggerStep(event Event) bool {
	if (p.triggers&Create == Create) && event.IsCreate() {
		return true
	}

	if (p.triggers&Modify == Modify) && event.IsModify() {
		return true
	}

	if (p.triggers&Delete == Delete) && event.IsDelete() {
		return true
	}

	if (p.triggers&Rename == Rename) && event.IsRename() {
		return true
	}

	return false
}

// patternStep discards events that don't match one of the shell file name patterns
func (p *pipeline) patternStep(event Event) bool {
	for _, pattern := range p.patterns {
		matched, err := filepath.Match(pattern, filepath.Base(event.Path()))
		// treat ErrBadPattern as a non-match:
		if err == nil && matched {
			return true
		}
	}
	if p.verbose {
		log.Printf("pattern %v not matched for %v", p.patterns, event)
	}
	return false
}

const throttleLatency = 1 * time.Second

// throttleStep
func (p *pipeline) throttleStep(event Event) bool {
	forward := true

	p.lastEventMutex.Lock()
	eventAt, ok := p.lastEventAt[event.Path()]
	if ok && time.Now().Sub(eventAt) <= throttleLatency {
		forward = false
	} else {
		p.lastEventAt[event.Path()] = time.Now()
	}
	p.lastEventMutex.Unlock()

	if p.verbose {
		log.Printf("thottle forward=%t for %v", forward, event)
	}
	return forward
}
