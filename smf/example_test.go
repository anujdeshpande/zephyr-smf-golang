// Copyright 2021 The Chromium OS Authors
// Copyright 2026 Anuj Deshpande
// SPDX-License-Identifier: Apache-2.0

// The examples from Zephyr's SMF documentation (doc/services/smf/index.rst).

package smf_test

import (
	"fmt"

	"github.com/anujdeshpande/zephyr-smf-golang/smf"
)

// This example turns the following flat state diagram into code, where the
// initial state is S0:
//
//	S0 --> S1 --> S2 --> S0
func Example_flat() {
	// List of demo states.
	const (
		s0 = iota
		s1
		s2
	)

	// User defined object.
	type object struct {
		smf.Ctx[*object]
		// Other state specific data goes here.
		steps int
	}

	states := make([]smf.State[*object], 3)
	states[s0] = smf.State[*object]{
		Entry: func(*object) { fmt.Println("s0 entry") },
		Run: func(o *object) smf.Result {
			o.SetState(&states[s1])
			return smf.Handled
		},
		Exit: func(*object) { fmt.Println("s0 exit") },
	}
	// State S1 does not have an entry action.
	states[s1] = smf.State[*object]{
		Run: func(o *object) smf.Result {
			o.SetState(&states[s2])
			return smf.Handled
		},
		Exit: func(*object) { fmt.Println("s1 exit") },
	}
	// State S2 does not have an exit action.
	states[s2] = smf.State[*object]{
		Entry: func(*object) { fmt.Println("s2 entry") },
		Run: func(o *object) smf.Result {
			o.SetState(&states[s0])
			return smf.Handled
		},
	}

	obj := &object{}
	obj.SetInitial(obj, &states[s0])

	// Run the state machine. It terminates if a non-zero value is
	// returned; this one never does, so stop after four steps.
	for ; obj.steps < 4; obj.steps++ {
		if ret := obj.RunState(); ret != 0 {
			break
		}
	}
	// Output:
	// s0 entry
	// s0 exit
	// s1 exit
	// s2 entry
	// s0 entry
	// s0 exit
}

// This example turns the following hierarchical state diagram into code,
// where S0 and S1 share a parent state and S0 is the initial state:
//
//	PARENT{S0 --> S1} --> S2 --> S0
//
// The parent's entry action runs before S0's, and its exit action after
// S1's. Moving between S0 and S1 runs neither.
func Example_hierarchical() {
	const (
		parent = iota
		s0
		s1
		s2
	)

	type object struct {
		smf.Ctx[*object]
	}

	states := make([]smf.State[*object], 4)
	run := func(name string, next int) func(*object) smf.Result {
		return func(o *object) smf.Result {
			fmt.Println(name, "run")
			o.SetState(&states[next])
			return smf.Handled
		}
	}

	// The parent state does not have a run action.
	states[parent] = smf.State[*object]{
		Entry: func(*object) { fmt.Println("parent entry") },
		Exit:  func(*object) { fmt.Println("parent exit") },
	}
	// The child states do not have entry or exit actions.
	states[s0] = smf.State[*object]{Run: run("s0", s1), Parent: &states[parent]}
	states[s1] = smf.State[*object]{Run: run("s1", s2), Parent: &states[parent]}
	// State S2 has no entry or exit actions and no parent.
	states[s2] = smf.State[*object]{
		Entry: func(*object) { fmt.Println("s2 entry") },
		Run:   run("s2", s0),
	}

	obj := &object{}
	obj.SetInitial(obj, &states[s0])
	for range 3 {
		obj.RunState()
	}
	// Output:
	// parent entry
	// s0 run
	// s1 run
	// parent exit
	// s2 entry
	// s2 run
	// parent entry
}

// Events are not part of the framework. An event driven state machine can
// be written by running one iteration per event received, here from a
// channel standing in for a button interrupt.
func Example_eventDriven() {
	const (
		s0 = iota
		s1
	)

	type event int
	const btnPress event = 1

	type object struct {
		smf.Ctx[*object]
		event event
	}

	states := make([]smf.State[*object], 2)
	states[s0] = smf.State[*object]{
		Entry: func(*object) { fmt.Println("STATE0") },
		Run: func(o *object) smf.Result {
			// Change states on a button press event.
			if o.event == btnPress {
				o.SetState(&states[s1])
			}
			return smf.Handled
		},
	}
	states[s1] = smf.State[*object]{
		Entry: func(*object) { fmt.Println("STATE1") },
		Run: func(o *object) smf.Result {
			if o.event == btnPress {
				o.SetState(&states[s0])
			}
			return smf.Handled
		},
	}

	events := make(chan event, 3)
	for range 3 {
		events <- btnPress
	}
	close(events)

	obj := &object{}
	obj.SetInitial(obj, &states[s0])
	// Block until an event arrives, then run the state machine once.
	for obj.event = range events {
		if ret := obj.RunState(); ret != 0 {
			break
		}
	}
	// Output:
	// STATE0
	// STATE1
	// STATE0
	// STATE1
}

// Hooks observe a state machine without changing it. SetHooks must be called
// after SetInitial, so the initial entry actions are not reported.
func ExampleCtx_SetHooks() {
	type object struct {
		smf.Ctx[*object]
	}

	var s0, s1 smf.State[*object]
	s0.Run = func(o *object) smf.Result {
		o.SetState(&s1)
		return smf.Handled
	}
	s1.Entry = func(o *object) { o.SetTerminate(1) }

	name := map[*smf.State[*object]]string{&s0: "s0", &s1: "s1"}
	hooks := &smf.Hooks[*object]{
		OnAction: func(_ *object, s *smf.State[*object], a smf.Action) {
			fmt.Println("action", name[s], a)
		},
		OnTransition: func(_ *object, source, dest *smf.State[*object]) {
			fmt.Println("transition", name[source], "->", name[dest])
		},
	}

	obj := &object{}
	obj.SetInitial(obj, &s0)
	obj.SetHooks(hooks)
	for obj.RunState() == 0 {
	}
	// Output:
	// action s0 run
	// transition s0 -> s1
	// action s1 entry
}
