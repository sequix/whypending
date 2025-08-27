package ctrlc

import (
	"os"
	"syscall"
	"testing"
)

func TestHandler(t *testing.T) {
	h := Handler()
	go func() { _ = syscall.Kill(os.Getpid(), syscall.SIGTERM) }()
	<-h
}
