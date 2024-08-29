package cmd

import (
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

// ErrorCode contains the exit code for server exit.
type ErrorCode struct {
	Code int
}

func (e ErrorCode) Error() string {
	return strconv.Itoa(e.Code)
}

// WaitForQuitSignals waits for SIGINT and SIGTERM and returns.
func WaitForQuitSignals() ErrorCode {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs
	return ErrorCode{Code: int(sig.(syscall.Signal)) + 128}
}
