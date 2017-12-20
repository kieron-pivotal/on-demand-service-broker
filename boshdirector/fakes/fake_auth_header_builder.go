// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"log"
	"net/http"
	"sync"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

type FakeAuthHeaderBuilder struct {
	AddAuthHeaderStub        func(request *http.Request, logger *log.Logger) error
	addAuthHeaderMutex       sync.RWMutex
	addAuthHeaderArgsForCall []struct {
		request *http.Request
		logger  *log.Logger
	}
	addAuthHeaderReturns struct {
		result1 error
	}
	addAuthHeaderReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeAuthHeaderBuilder) AddAuthHeader(request *http.Request, logger *log.Logger) error {
	fake.addAuthHeaderMutex.Lock()
	ret, specificReturn := fake.addAuthHeaderReturnsOnCall[len(fake.addAuthHeaderArgsForCall)]
	fake.addAuthHeaderArgsForCall = append(fake.addAuthHeaderArgsForCall, struct {
		request *http.Request
		logger  *log.Logger
	}{request, logger})
	fake.recordInvocation("AddAuthHeader", []interface{}{request, logger})
	fake.addAuthHeaderMutex.Unlock()
	if fake.AddAuthHeaderStub != nil {
		return fake.AddAuthHeaderStub(request, logger)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.addAuthHeaderReturns.result1
}

func (fake *FakeAuthHeaderBuilder) AddAuthHeaderCallCount() int {
	fake.addAuthHeaderMutex.RLock()
	defer fake.addAuthHeaderMutex.RUnlock()
	return len(fake.addAuthHeaderArgsForCall)
}

func (fake *FakeAuthHeaderBuilder) AddAuthHeaderArgsForCall(i int) (*http.Request, *log.Logger) {
	fake.addAuthHeaderMutex.RLock()
	defer fake.addAuthHeaderMutex.RUnlock()
	return fake.addAuthHeaderArgsForCall[i].request, fake.addAuthHeaderArgsForCall[i].logger
}

func (fake *FakeAuthHeaderBuilder) AddAuthHeaderReturns(result1 error) {
	fake.AddAuthHeaderStub = nil
	fake.addAuthHeaderReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeAuthHeaderBuilder) AddAuthHeaderReturnsOnCall(i int, result1 error) {
	fake.AddAuthHeaderStub = nil
	if fake.addAuthHeaderReturnsOnCall == nil {
		fake.addAuthHeaderReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.addAuthHeaderReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeAuthHeaderBuilder) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.addAuthHeaderMutex.RLock()
	defer fake.addAuthHeaderMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeAuthHeaderBuilder) recordInvocation(key string, args []interface{}) {
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

var _ boshdirector.AuthHeaderBuilder = new(FakeAuthHeaderBuilder)