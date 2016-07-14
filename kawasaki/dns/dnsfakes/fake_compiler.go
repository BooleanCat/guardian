// This file was generated by counterfeiter
package dnsfakes

import (
	"sync"

	"code.cloudfoundry.org/guardian/kawasaki/dns"
	"code.cloudfoundry.org/lager"
)

type FakeCompiler struct {
	CompileStub        func(log lager.Logger) ([]byte, error)
	compileMutex       sync.RWMutex
	compileArgsForCall []struct {
		log lager.Logger
	}
	compileReturns struct {
		result1 []byte
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeCompiler) Compile(log lager.Logger) ([]byte, error) {
	fake.compileMutex.Lock()
	fake.compileArgsForCall = append(fake.compileArgsForCall, struct {
		log lager.Logger
	}{log})
	fake.recordInvocation("Compile", []interface{}{log})
	fake.compileMutex.Unlock()
	if fake.CompileStub != nil {
		return fake.CompileStub(log)
	} else {
		return fake.compileReturns.result1, fake.compileReturns.result2
	}
}

func (fake *FakeCompiler) CompileCallCount() int {
	fake.compileMutex.RLock()
	defer fake.compileMutex.RUnlock()
	return len(fake.compileArgsForCall)
}

func (fake *FakeCompiler) CompileArgsForCall(i int) lager.Logger {
	fake.compileMutex.RLock()
	defer fake.compileMutex.RUnlock()
	return fake.compileArgsForCall[i].log
}

func (fake *FakeCompiler) CompileReturns(result1 []byte, result2 error) {
	fake.CompileStub = nil
	fake.compileReturns = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakeCompiler) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.compileMutex.RLock()
	defer fake.compileMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeCompiler) recordInvocation(key string, args []interface{}) {
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

var _ dns.Compiler = new(FakeCompiler)
