// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

type FakeAuthenticatorBuilder struct {
	NewAuthHeaderBuilderStub        func(boshInfo boshdirector.Info, disableSSLCertVerification bool) (boshdirector.AuthHeaderBuilder, error)
	newAuthHeaderBuilderMutex       sync.RWMutex
	newAuthHeaderBuilderArgsForCall []struct {
		boshInfo                   boshdirector.Info
		disableSSLCertVerification bool
	}
	newAuthHeaderBuilderReturns struct {
		result1 boshdirector.AuthHeaderBuilder
		result2 error
	}
	newAuthHeaderBuilderReturnsOnCall map[int]struct {
		result1 boshdirector.AuthHeaderBuilder
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeAuthenticatorBuilder) NewAuthHeaderBuilder(boshInfo boshdirector.Info, disableSSLCertVerification bool) (boshdirector.AuthHeaderBuilder, error) {
	fake.newAuthHeaderBuilderMutex.Lock()
	ret, specificReturn := fake.newAuthHeaderBuilderReturnsOnCall[len(fake.newAuthHeaderBuilderArgsForCall)]
	fake.newAuthHeaderBuilderArgsForCall = append(fake.newAuthHeaderBuilderArgsForCall, struct {
		boshInfo                   boshdirector.Info
		disableSSLCertVerification bool
	}{boshInfo, disableSSLCertVerification})
	fake.recordInvocation("NewAuthHeaderBuilder", []interface{}{boshInfo, disableSSLCertVerification})
	fake.newAuthHeaderBuilderMutex.Unlock()
	if fake.NewAuthHeaderBuilderStub != nil {
		return fake.NewAuthHeaderBuilderStub(boshInfo, disableSSLCertVerification)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fake.newAuthHeaderBuilderReturns.result1, fake.newAuthHeaderBuilderReturns.result2
}

func (fake *FakeAuthenticatorBuilder) NewAuthHeaderBuilderCallCount() int {
	fake.newAuthHeaderBuilderMutex.RLock()
	defer fake.newAuthHeaderBuilderMutex.RUnlock()
	return len(fake.newAuthHeaderBuilderArgsForCall)
}

func (fake *FakeAuthenticatorBuilder) NewAuthHeaderBuilderArgsForCall(i int) (boshdirector.Info, bool) {
	fake.newAuthHeaderBuilderMutex.RLock()
	defer fake.newAuthHeaderBuilderMutex.RUnlock()
	return fake.newAuthHeaderBuilderArgsForCall[i].boshInfo, fake.newAuthHeaderBuilderArgsForCall[i].disableSSLCertVerification
}

func (fake *FakeAuthenticatorBuilder) NewAuthHeaderBuilderReturns(result1 boshdirector.AuthHeaderBuilder, result2 error) {
	fake.NewAuthHeaderBuilderStub = nil
	fake.newAuthHeaderBuilderReturns = struct {
		result1 boshdirector.AuthHeaderBuilder
		result2 error
	}{result1, result2}
}

func (fake *FakeAuthenticatorBuilder) NewAuthHeaderBuilderReturnsOnCall(i int, result1 boshdirector.AuthHeaderBuilder, result2 error) {
	fake.NewAuthHeaderBuilderStub = nil
	if fake.newAuthHeaderBuilderReturnsOnCall == nil {
		fake.newAuthHeaderBuilderReturnsOnCall = make(map[int]struct {
			result1 boshdirector.AuthHeaderBuilder
			result2 error
		})
	}
	fake.newAuthHeaderBuilderReturnsOnCall[i] = struct {
		result1 boshdirector.AuthHeaderBuilder
		result2 error
	}{result1, result2}
}

func (fake *FakeAuthenticatorBuilder) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.newAuthHeaderBuilderMutex.RLock()
	defer fake.newAuthHeaderBuilderMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeAuthenticatorBuilder) recordInvocation(key string, args []interface{}) {
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

var _ boshdirector.AuthenticatorBuilder = new(FakeAuthenticatorBuilder)
