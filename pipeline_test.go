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

func (e *fakeEvent) IsCreate() bool   { return e.create }
func (e *fakeEvent) IsDelete() bool   { return e.delete }
func (e *fakeEvent) IsModify() bool   { return e.modify }
func (e *fakeEvent) IsRename() bool   { return e.rename }
func (e *fakeEvent) fileName() string { return e.name }
func (e *fakeEvent) String() string   { return e.description }

var (
	createEvent = &fakeEvent{create: true, description: "Create"}
	deleteEvent = &fakeEvent{delete: true, description: "Delete"}
	modifyEvent = &fakeEvent{modify: true, description: "Modify"}
	renameEvent = &fakeEvent{rename: true, description: "Rename"}
)

func TestTriggerAllEventsFiltersNothing(t *testing.T) {
	p := newPipeline(FSN_ALL)

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
	p := newPipeline(FSN_DELETE)

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
	p := newPipeline(FSN_CREATE | FSN_MODIFY)

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
