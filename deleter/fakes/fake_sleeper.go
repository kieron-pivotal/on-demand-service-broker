// This file was generated by counterfeiter
package fakes

import (
	"sync"
	"time"

	"github.com/pivotal-cf/on-demand-service-broker/deleter"
)

type FakeSleeper struct {
	SleepStub        func(d time.Duration)
	sleepMutex       sync.RWMutex
	sleepArgsForCall []struct {
		d time.Duration
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeSleeper) Sleep(d time.Duration) {
	fake.sleepMutex.Lock()
	fake.sleepArgsForCall = append(fake.sleepArgsForCall, struct {
		d time.Duration
	}{d})
	fake.recordInvocation("Sleep", []interface{}{d})
	fake.sleepMutex.Unlock()
	if fake.SleepStub != nil {
		fake.SleepStub(d)
	}
}

func (fake *FakeSleeper) SleepCallCount() int {
	fake.sleepMutex.RLock()
	defer fake.sleepMutex.RUnlock()
	return len(fake.sleepArgsForCall)
}

func (fake *FakeSleeper) SleepArgsForCall(i int) time.Duration {
	fake.sleepMutex.RLock()
	defer fake.sleepMutex.RUnlock()
	return fake.sleepArgsForCall[i].d
}

func (fake *FakeSleeper) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.sleepMutex.RLock()
	defer fake.sleepMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeSleeper) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ deleter.Sleeper = new(FakeSleeper)