// Copyright 2026 Anuj Deshpande
// SPDX-License-Identifier: Apache-2.0

// Tests beyond the Zephyr suite: the two errors reported through the
// OnError hook, Previous, and Executing.

package smf_test

import (
	"errors"
	"testing"

	"github.com/anujdeshpande/zephyr-smf-golang/smf"
)

type errObject struct {
	smf.Ctx[*errObject]
	errs   []error
	target *smf.State[*errObject]
}

func (o *errObject) hooks() *smf.Hooks[*errObject] {
	return &smf.Hooks[*errObject]{
		OnError: func(o *errObject, err error) { o.errs = append(o.errs, err) },
	}
}

func TestSetStateNil(t *testing.T) {
	var a smf.State[*errObject]
	a.Run = func(o *errObject) smf.Result {
		o.SetState(nil)
		return smf.Handled
	}

	o := &errObject{}
	o.SetInitial(o, &a)
	o.SetHooks(o.hooks())
	if rc := o.RunState(); rc != 0 {
		t.Fatalf("RunState = %d, want 0", rc)
	}
	if len(o.errs) != 1 || !errors.Is(o.errs[0], smf.ErrNilTransition) {
		t.Errorf("errors = %v, want [ErrNilTransition]", o.errs)
	}
	if o.Current() != &a {
		t.Error("state changed after SetState(nil)")
	}
}

func TestSetStateInExit(t *testing.T) {
	var a, b, c smf.State[*errObject]
	a.Run = func(o *errObject) smf.Result {
		o.SetState(&b)
		return smf.Handled
	}
	a.Exit = func(o *errObject) { o.SetState(&c) }

	o := &errObject{}
	o.SetInitial(o, &a)
	o.SetHooks(o.hooks())
	o.RunState()

	if len(o.errs) != 1 || !errors.Is(o.errs[0], smf.ErrTransitionInExit) {
		t.Errorf("errors = %v, want [ErrTransitionInExit]", o.errs)
	}
	if o.Current() != &b {
		t.Error("SetState in an exit action changed the transition target")
	}
	if o.Previous() != &a {
		t.Error("Previous is not the state the transition left")
	}
}

func TestExecuting(t *testing.T) {
	var parent, child smf.State[*errObject]
	child.Parent = &parent
	var seen []*smf.State[*errObject]
	record := func(o *errObject) smf.Result {
		seen = append(seen, o.Executing())
		return smf.Propagate
	}
	parent.Run = record
	child.Run = record

	o := &errObject{}
	o.SetInitial(o, &child)
	o.RunState()

	if len(seen) != 2 || seen[0] != &child || seen[1] != &parent {
		t.Errorf("executing states = %v, want [child parent]", seen)
	}
	if o.Executing() != &child {
		t.Error("Executing after RunState is not the current state")
	}
}

func TestActionString(t *testing.T) {
	for a, want := range map[smf.Action]string{
		smf.ActionEntry: "entry",
		smf.ActionRun:   "run",
		smf.ActionExit:  "exit",
		smf.Action(7):   "Action(7)",
	} {
		if got := a.String(); got != want {
			t.Errorf("Action(%d).String() = %q, want %q", int(a), got, want)
		}
	}
}
