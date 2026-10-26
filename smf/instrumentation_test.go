// SPDX-FileCopyrightText: Copyright The Zephyr Project Contributors
// Copyright 2026 Anuj Deshpande
// SPDX-License-Identifier: Apache-2.0

// Common test instrumentation for the instrumented SMF tests, ported from
// tests/lib/smf/src/test_instrumentation_common.h. It records actions and
// transitions through the hooks to verify state machine behavior.

package smf_test

import (
	"testing"

	"github.com/anujdeshpande/zephyr-smf-golang/smf"
)

type actionRecord[T any] struct {
	state  *smf.State[T]
	action smf.Action
}

type transitionRecord[T any] struct {
	source, dest *smf.State[T]
}

// recorder plays the part of the static logs in the C header. A new
// recorder per run replaces reset_logs.
type recorder[T any] struct {
	actions     []actionRecord[T]
	transitions []transitionRecord[T]
	errors      int
}

func (r *recorder[T]) hooks() *smf.Hooks[T] {
	return &smf.Hooks[T]{
		OnTransition: func(_ T, source, dest *smf.State[T]) {
			r.transitions = append(r.transitions, transitionRecord[T]{source, dest})
		},
		OnAction: func(_ T, state *smf.State[T], action smf.Action) {
			r.actions = append(r.actions, actionRecord[T]{state, action})
		},
		OnError: func(T, error) {
			r.errors++
		},
	}
}

// stateName names a state in failure messages.
type stateName[T any] func(*smf.State[T]) string

func (r *recorder[T]) checkActions(t *testing.T, name stateName[T], want []actionRecord[T]) {
	t.Helper()
	if len(r.actions) != len(want) {
		t.Errorf("got %d action hooks, want %d", len(r.actions), len(want))
	}
	for i := range min(len(r.actions), len(want)) {
		got := r.actions[i]
		if got.state != want[i].state {
			t.Errorf("action %d: state %s, want %s", i, name(got.state), name(want[i].state))
		}
		if got.action != want[i].action {
			t.Errorf("action %d: action type %v, want %v", i, got.action, want[i].action)
		}
	}
}

func (r *recorder[T]) checkTransitions(t *testing.T, name stateName[T], want []transitionRecord[T]) {
	t.Helper()
	if len(r.transitions) != len(want) {
		t.Errorf("got %d transitions, want %d", len(r.transitions), len(want))
	}
	for i := range min(len(r.transitions), len(want)) {
		got := r.transitions[i]
		if got.source != want[i].source || got.dest != want[i].dest {
			t.Errorf("transition %d: %s -> %s, want %s -> %s", i,
				name(got.source), name(got.dest), name(want[i].source), name(want[i].dest))
		}
	}
}

func (r *recorder[T]) checkErrors(t *testing.T, want int) {
	t.Helper()
	if r.errors != want {
		t.Errorf("got %d errors, want %d", r.errors, want)
	}
}

// indexName names a state by its index in a state table.
func indexName[T any](table []smf.State[T], names []string) stateName[T] {
	return func(s *smf.State[T]) string {
		if s == nil {
			return "<nil>"
		}
		for i := range table {
			if &table[i] == s {
				return names[i]
			}
		}
		return "<unknown>"
	}
}
