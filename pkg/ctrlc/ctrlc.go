package ctrlc

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var (
	onlyOneHandler = make(chan struct{})
)

// Handler registered for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals.
func Handler() <-chan struct{} {
	close(onlyOneHandler) // panics when called twice

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		close(stop)
		<-c
		fmt.Println("exit because received twice terminal signals")
		os.Exit(1)
	}()

	return stop
}
