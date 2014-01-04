// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fsnotify

import (
	"testing"
)

type fakeEvent struct {
	create      bool
	delete      bool
	modify      bool
	rename      bool
	name        string
	description string // just for testing
}

func (e *fakeEvent) IsCreate() bool { return e.create }
func (e *fakeEvent) IsDelete() bool { return e.delete }
func (e *fakeEvent) IsModify() bool { return e.modify }
func (e *fakeEvent) IsRename() bool { return e.rename }
func (e *fakeEvent) Path() string   { return e.name }
func (e *fakeEvent) String() string { return e.description }

/*
  Triggers option
*/
var (
	createEvent = &fakeEvent{create: true, description: "Create"}
	deleteEvent = &fakeEvent{delete: true, description: "Delete"}
	modifyEvent = &fakeEvent{modify: true, description: "Modify"}
	renameEvent = &fakeEvent{rename: true, description: "Rename"}
)

func TestAllTriggersFiltersNothing(t *testing.T) {
	p := newPipeline(&Options{Triggers: allTriggers})

	var tests = []struct {
		event   *fakeEvent
		forward bool
	}{
		{createEvent, true},
		{deleteEvent, true},
		{modifyEvent, true},
		{renameEvent, true},
	}

	for index, tt := range tests {
		if forward := p.processEvent(tt.event); forward != tt.forward {
			t.Errorf("%d. %v event should be forwarded for FSN_ALL", index, tt.event)
		}
	}
}

func TestTriggerDeleteFiltersOtherEvents(t *testing.T) {
	p := newPipeline(&Options{Triggers: Delete})

	var tests = []struct {
		event   *fakeEvent
		forward bool
	}{
		{createEvent, false},
		{deleteEvent, true},
		{modifyEvent, false},
		{renameEvent, false},
	}

	for index, tt := range tests {
		if forward := p.processEvent(tt.event); forward != tt.forward {
			t.Errorf("%d. %v event for FSN_DELETE, want forward=%t got %t", index, tt.event, tt.forward, forward)
		}
	}
}

func TestTriggerCreateModifyFiltersOtherEvents(t *testing.T) {
	p := newPipeline(&Options{Triggers: Create | Modify})

	var tests = []struct {
		event   *fakeEvent
		forward bool
	}{
		{createEvent, true},
		{deleteEvent, false},
		{modifyEvent, true},
		{renameEvent, false},
	}

	for index, tt := range tests {
		if forward := p.processEvent(tt.event); forward != tt.forward {
			t.Errorf("%d. %v event for FSN_CREATE, want forward=%t got %t", index, tt.event, tt.forward, forward)
		}
	}
}

/*
  Hidden option
*/
var (
	hiddenEvent         = &fakeEvent{create: true, name: ".subl26d.tmp", description: "hidden file"}
	visibleEvent        = &fakeEvent{create: true, name: "main.go", description: "visible file"}
	hiddenInFolderEvent = &fakeEvent{create: true, name: "folder/.DS_Store", description: "folder/.hidden file"}
)

func TestHiddenFiltersHiddenEvent(t *testing.T) {
	p := newPipeline(&Options{Hidden: false})

	if forward := p.processEvent(hiddenEvent); forward != false {
		t.Errorf("Hidden should filter %v event, want forward=%t got %t", hiddenEvent, false, forward)
	}
	if forward := p.processEvent(hiddenInFolderEvent); forward != false {
		t.Errorf("Hidden should filter %v event, want forward=%t got %t", hiddenInFolderEvent, false, forward)
	}
	if forward := p.processEvent(visibleEvent); forward != true {
		t.Errorf("Hidden should not filter %v, want forward=%t got %t", visibleEvent, true, forward)
	}
}

func TestHiddenIncludesHiddenEvent(t *testing.T) {
	p := newPipeline(&Options{Hidden: true})

	if forward := p.processEvent(hiddenEvent); forward != true {
		t.Errorf("Include hidden should not filter %v event, want forward=%t got %t", hiddenEvent, true, forward)
	}
}

/*
  Pattern
*/
var (
	goEvent        = &fakeEvent{create: true, name: "main.go", description: "go file"}
	cEvent         = &fakeEvent{create: true, name: "main.c", description: "c file"}
	mdEvent        = &fakeEvent{create: true, name: "README.md", description: "markdown file"}
	goInFolerEvent = &fakeEvent{create: true, name: "folder/main.go", description: "folder/go file"}
)

func TestNoPattern(t *testing.T) {
	p := newPipeline(&Options{Pattern: ""})
	if p.processEvent(cEvent) != true {
		t.Errorf("No pattern should forward %v", cEvent)
	}
}

func TestSinglePattern(t *testing.T) {
	p := newPipeline(&Options{Pattern: "*.go"})

	if forward := p.processEvent(goEvent); forward != true {
		t.Errorf("*.go pattern should forward %v", goEvent)
	}

	if forward := p.processEvent(cEvent); forward != false {
		t.Errorf("*.go pattern should not forward %v", cEvent)
	}

	if forward := p.processEvent(goInFolerEvent); forward != true {
		t.Errorf("*.go pattern should forward %v", goInFolerEvent)
	}
}

func TestMultiplePatterns(t *testing.T) {
	p := newPipeline(&Options{Pattern: "*.go,*.c"})

	if forward := p.processEvent(goEvent); forward != true {
		t.Errorf("*.go,*.c pattern should forward %v", goEvent)
	}

	if forward := p.processEvent(cEvent); forward != true {
		t.Errorf("*.go,*.c pattern should forward %v", cEvent)
	}

	if forward := p.processEvent(mdEvent); forward != false {
		t.Errorf("*.go,*.c pattern should not forward %v", mdEvent)
	}
}

/*
  Throttle
*/
func TestThrottleSameEvent(t *testing.T) {
	p := newPipeline(&Options{Throttle: true})

	if forward := p.processEvent(goEvent); forward != true {
		t.Errorf("Throttle should forward %v event on leading edge", goEvent)
	}
	if forward := p.processEvent(goEvent); forward != false {
		t.Errorf("Throttle should not forward %v event a second time", goEvent)
	}
	// TODO: it should forward again after latency time
}

func TestThrottleDifferentEvents(t *testing.T) {
	p := newPipeline(&Options{Throttle: true})

	if forward := p.processEvent(goEvent); forward != true {
		t.Errorf("Throttle should forward %v event", goEvent)
	}
	if forward := p.processEvent(cEvent); forward != true {
		t.Errorf("Throttle should forward %v event", cEvent)
	}
}

/*
  Autowatch
*/
