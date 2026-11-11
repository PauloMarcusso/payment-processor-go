package fsm_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pmarcusso/payment-processor-go/pkg/fsm"
)

type payload struct {
	visited string
	score   float64
}

func record(p *payload, name string) {
	p.visited += name
}

func TestRunHappyPath(t *testing.T) {
	m := fsm.New[payload]("start").
		Handle("start", func(_ context.Context, p *payload) (fsm.State, error) {
			record(p, "start")
			return "score", nil
		}).
		Handle("score", func(_ context.Context, p *payload) (fsm.State, error) {
			record(p, "score")
			p.score = 0.9
			return "done", nil
		}).
		Terminal("done")

	var p payload
	trace, err := m.Run(context.Background(), &p)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if len(trace) != 2 {
		t.Errorf("len(trace) = %d, want 2", len(trace))
	}
	if p.score != 0.9 {
		t.Errorf("p.score = %v, want 0.9", p.score)
	}
}

func TestRunHandlerError(t *testing.T) {
	boom := errors.New("boom")
	failing := func(context.Context, *payload) (fsm.State, error) { return "", boom }
	finalize := func(_ context.Context, p *payload) (fsm.State, error) {
		record(p, "finalize")
		return "done", nil
	}

	tests := []struct {
		name        string
		machine     *fsm.Machine[payload]
		wantErr     error
		wantVisited string
		wantTrace   []fsm.State
	}{
		{
			name: "fallback routes to target",
			machine: fsm.New[payload]("ml").
				Handle("ml", failing).
				Handle("finalize", finalize).
				FallbackTo("ml", "finalize").
				Terminal("done"),
			wantVisited: "finalize",
			wantTrace:   []fsm.State{"finalize", "done"},
		},
		{
			name: "no fallback aborts with the handler error",
			machine: fsm.New[payload]("ml").
				Handle("ml", failing).
				Terminal("done"),
			wantErr:   boom,
			wantTrace: []fsm.State{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p payload
			trace, err := tt.machine.Run(context.Background(), &p)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Run() error = %v, want %v", err, tt.wantErr)
			}
			if p.visited != tt.wantVisited {
				t.Errorf("visited = %q, want %q", p.visited, tt.wantVisited)
			}
			if len(trace) != len(tt.wantTrace) {
				t.Fatalf("len(trace) = %d, want %d: %+v", len(trace), len(tt.wantTrace), trace)
			}
			for i, to := range tt.wantTrace {
				if trace[i].To != to {
					t.Errorf("trace[%d].To = %q, want %q", i, trace[i].To, to)
				}
			}
		})
	}
}

func TestFallbackKeepsPipelineAvailable(t *testing.T) {
	dependencyDown := errors.New("upstream timeout")

	m := fsm.New[payload]("start").
		Handle("start", func(_ context.Context, p *payload) (fsm.State, error) {
			record(p, "start")
			return "ml", nil
		}).
		Handle("ml", func(context.Context, *payload) (fsm.State, error) {
			return "", dependencyDown
		}).
		FallbackTo("ml", "rules_only").
		Handle("rules_only", func(_ context.Context, p *payload) (fsm.State, error) {
			record(p, "rules_only")
			return "done", nil
		}).
		Terminal("done")

	var p payload
	trace, err := m.Run(context.Background(), &p)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil (fallback absorbs the failure)", err)
	}
	if !strings.HasSuffix(p.visited, "rules_only") {
		t.Errorf("visited = %q, want suffix %q", p.visited, "rules_only")
	}
	if len(trace) < 2 || !errors.Is(trace[1].Err, dependencyDown) {
		t.Errorf("trace = %+v, want trace[1].Err to preserve %v", trace, dependencyDown)
	}
}

func TestErrorWithoutFallbackAborts(t *testing.T) {
	boom := errors.New("boom")

	m := fsm.New[payload]("start").
		Handle("start", func(context.Context, *payload) (fsm.State, error) {
			return "", boom
		}).
		Terminal("done")

	var p payload
	_, err := m.Run(context.Background(), &p)
	if !errors.Is(err, boom) {
		t.Errorf("Run() error = %v, want errors.Is(err, boom)", err)
	}
}
