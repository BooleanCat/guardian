// This file was generated by counterfeiter
package fakes

import (
	"sync"

	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"
)

type FakeEventsNotifier struct {
	OnEventStub        func(handle string, event string)
	onEventMutex       sync.RWMutex
	onEventArgsForCall []struct {
		handle string
		event  string
	}
}

func (fake *FakeEventsNotifier) OnEvent(handle string, event string) {
	fake.onEventMutex.Lock()
	fake.onEventArgsForCall = append(fake.onEventArgsForCall, struct {
		handle string
		event  string
	}{handle, event})
	fake.onEventMutex.Unlock()
	if fake.OnEventStub != nil {
		fake.OnEventStub(handle, event)
	}
}

func (fake *FakeEventsNotifier) OnEventCallCount() int {
	fake.onEventMutex.RLock()
	defer fake.onEventMutex.RUnlock()
	return len(fake.onEventArgsForCall)
}

func (fake *FakeEventsNotifier) OnEventArgsForCall(i int) (string, string) {
	fake.onEventMutex.RLock()
	defer fake.onEventMutex.RUnlock()
	return fake.onEventArgsForCall[i].handle, fake.onEventArgsForCall[i].event
}

var _ runrunc.EventsNotifier = new(FakeEventsNotifier)