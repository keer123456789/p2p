package main

import (
	"github.com/stretchr/testify/assert"
	"os"
	"syscall"
	"testing"
)

func TestNewSignalSet(t *testing.T) {
	assert := assert.New(t)
	ss := NewSignalSet()
	assert.NotNil(ss)
}

var sigint bool = false

func sigintHandler(s os.Signal, arg interface{}) {
	sigint = true
	return
}

func TestSignalSet_RegisterSysSignal(t *testing.T) {
	assert := assert.New(t)
	ss := NewSignalSet()
	ss.RegisterSysSignal(syscall.SIGINT, sigintHandler)
	handler := ss.m[syscall.SIGINT]
	assert.NotNil(handler)
}
