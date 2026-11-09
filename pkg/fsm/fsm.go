// Package fsm provides a minimal deterministic finite state machine.
// It/s designed for orchestrating multi-step decision pipelines where
// every transation must be explicit and observable
package fsm

import (
	"context"
	"errors"
	"time"
)

type State string

var ErrNoTransition = errors.New("fsm: no transation registered for state")

type Handler[T any] func(ctx context.Context, data *T) (State, error)

type Transition struct {
	From     State
	To       State
	Duration time.Duration
	Err      error
}
