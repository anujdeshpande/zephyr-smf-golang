// Copyright 2021 The Chromium OS Authors
// Copyright 2026 Anuj Deshpande
// SPDX-License-Identifier: Apache-2.0

// Ported from tests/lib/smf/src/test_lib_flat_smf.c.
//
// Flat Test Transition:
//
//	A_ENTRY --> A_RUN --> A_EXIT --> B_ENTRY --> B_RUN --|
//	                                                     |
//	|----------------------------------------------------|
//	|
//	|--> B_EXIT --> C_ENTRY --> C_RUN --> C_EXIT --> D_ENTRY
//
// The instrumentation hooks verify the exact sequence of actions and
// transitions.
//
// Note: SetInitial clears the hooks, so they are installed after
// initialisation. Initial entry actions are therefore NOT captured in the
// trace.

package smf_test

import (
	"testing"

	"github.com/anujdeshpande/zephyr-smf-golang/smf"
)

const flatSMFRun = 3

// List of all TypeC-level states.
const (
	flatStateA = iota
	flatStateB
	flatStateC
	flatStateD
)

var flatNames = []string{"STATE_A", "STATE_B", "STATE_C", "STATE_D"}

type flatTerminateAction int

const (
	flatNone flatTerminateAction = iota
	flatEntry
	flatRun
	flatExit
)

type flatObject struct {
	smf.Ctx[*flatObject]
	terminate flatTerminateAction
}

var flatStates []smf.State[*flatObject]

func init() {
	flatStates = []smf.State[*flatObject]{
		flatStateA: {Entry: flatStateAEntry, Run: flatStateARun, Exit: flatStateAExit},
		flatStateB: {Entry: flatStateBEntry, Run: flatStateBRun, Exit: flatStateBExit},
		flatStateC: {Entry: flatStateCEntry, Run: flatStateCRun, Exit: flatStateCExit},
		flatStateD: {Entry: flatStateDEntry, Run: flatStateDRun, Exit: flatStateDExit},
	}
}

// State A handlers

func flatStateAEntry(o *flatObject) {
	if o.terminate == flatEntry {
		o.SetTerminate(-1)
	}
}

func flatStateARun(o *flatObject) smf.Result {
	o.SetState(&flatStates[flatStateB])
	return smf.Handled
}

func flatStateAExit(*flatObject) {}

// State B handlers

func flatStateBEntry(*flatObject) {}

func flatStateBRun(o *flatObject) smf.Result {
	if o.terminate == flatRun {
		o.SetTerminate(-1)
		return smf.Handled
	}

	o.SetState(&flatStates[flatStateC])
	return smf.Handled
}

func flatStateBExit(*flatObject) {}

// State C handlers

func flatStateCEntry(*flatObject) {}

func flatStateCRun(o *flatObject) smf.Result {
	o.SetState(&flatStates[flatStateD])
	return smf.Handled
}

func flatStateCExit(o *flatObject) {
	if o.terminate == flatExit {
		o.SetTerminate(-1)
	}
}

// State D handlers

func flatStateDEntry(*flatObject) {}

func flatStateDRun(*flatObject) smf.Result { return smf.Handled }

func flatStateDExit(*flatObject) {}

// runFlatFSM runs the FSM with hooks installed after SetInitial. Because
// SetInitial clears the hooks, initial entry actions are NOT captured.
func runFlatFSM(initial int, term flatTerminateAction) (*flatObject, *recorder[*flatObject]) {
	rec := &recorder[*flatObject]{}
	obj := &flatObject{terminate: term}
	obj.SetInitial(obj, &flatStates[initial])
	obj.SetHooks(rec.hooks())

	for range flatSMFRun {
		if obj.RunState() != 0 {
			break
		}
	}
	return obj, rec
}

func flatAction(state int, action smf.Action) actionRecord[*flatObject] {
	return actionRecord[*flatObject]{&flatStates[state], action}
}

func flatTransition(source, dest int) transitionRecord[*flatObject] {
	return transitionRecord[*flatObject]{&flatStates[source], &flatStates[dest]}
}

// Flat test: normal transitions A -> B -> C -> D
//
// SetInitial(A): A_ENTRY (not captured, hooks installed after init)
//
// Run 0: A_RUN -> [A transitions to B] -> A_EXIT, transition(A,B), B_ENTRY
// Run 1: B_RUN -> [B transitions to C] -> B_EXIT, transition(B,C), C_ENTRY
// Run 2: C_RUN -> [C transitions to D] -> C_EXIT, transition(C,D), D_ENTRY
func TestSMFFlatInstrumentedNormal(t *testing.T) {
	obj, rec := runFlatFSM(flatStateA, flatNone)
	name := indexName(flatStates, flatNames)

	// Verify action sequence (initial A_ENTRY not captured).
	rec.checkActions(t, name, []actionRecord[*flatObject]{
		// Run 0: A_RUN, A_EXIT, B_ENTRY
		flatAction(flatStateA, smf.ActionRun),
		flatAction(flatStateA, smf.ActionExit),
		flatAction(flatStateB, smf.ActionEntry),
		// Run 1: B_RUN, B_EXIT, C_ENTRY
		flatAction(flatStateB, smf.ActionRun),
		flatAction(flatStateB, smf.ActionExit),
		flatAction(flatStateC, smf.ActionEntry),
		// Run 2: C_RUN, C_EXIT, D_ENTRY
		flatAction(flatStateC, smf.ActionRun),
		flatAction(flatStateC, smf.ActionExit),
		flatAction(flatStateD, smf.ActionEntry),
	})

	// Verify transition sequence.
	rec.checkTransitions(t, name, []transitionRecord[*flatObject]{
		flatTransition(flatStateA, flatStateB),
		flatTransition(flatStateB, flatStateC),
		flatTransition(flatStateC, flatStateD),
	})

	// Verify no errors.
	rec.checkErrors(t, 0)

	// Verify final state.
	if got := obj.Current(); got != &flatStates[flatStateD] {
		t.Errorf("final state %s, want STATE_D", name(got))
	}
}

// Flat test: terminate in entry action of State A
//
// SetInitial(A): flatStateAEntry terminates (hooks not installed yet).
// RunState returns immediately due to the terminate flag. No actions or
// transitions are captured.
func TestSMFFlatInstrumentedEntryTerminate(t *testing.T) {
	_, rec := runFlatFSM(flatStateA, flatEntry)
	name := indexName(flatStates, flatNames)

	// No actions captured: terminate during init.
	rec.checkActions(t, name, nil)
	// No transitions should occur.
	rec.checkTransitions(t, name, nil)
	rec.checkErrors(t, 0)
}

// Flat test: terminate in run action of State B
//
// SetInitial(A): A_ENTRY (not captured)
// Run 0: A_RUN -> [A transitions to B] -> A_EXIT, transition(A,B), B_ENTRY
// Run 1: B_RUN (terminates)
func TestSMFFlatInstrumentedRunTerminate(t *testing.T) {
	_, rec := runFlatFSM(flatStateA, flatRun)
	name := indexName(flatStates, flatNames)

	// A_RUN, A_EXIT, B_ENTRY, B_RUN = 4 actions (initial A_ENTRY not captured)
	rec.checkActions(t, name, []actionRecord[*flatObject]{
		flatAction(flatStateA, smf.ActionRun),
		flatAction(flatStateA, smf.ActionExit),
		flatAction(flatStateB, smf.ActionEntry),
		flatAction(flatStateB, smf.ActionRun),
	})

	rec.checkTransitions(t, name, []transitionRecord[*flatObject]{
		flatTransition(flatStateA, flatStateB),
	})

	rec.checkErrors(t, 0)
}

// Flat test: terminate in exit action of State C
//
// SetInitial(A): A_ENTRY (not captured)
// Run 0: A_RUN, A_EXIT, transition(A,B), B_ENTRY
// Run 1: B_RUN, B_EXIT, transition(B,C), C_ENTRY
// Run 2: C_RUN, C_EXIT (terminates during exit)
func TestSMFFlatInstrumentedExitTerminate(t *testing.T) {
	_, rec := runFlatFSM(flatStateA, flatExit)
	name := indexName(flatStates, flatNames)

	rec.checkActions(t, name, []actionRecord[*flatObject]{
		flatAction(flatStateA, smf.ActionRun),
		flatAction(flatStateA, smf.ActionExit),
		flatAction(flatStateB, smf.ActionEntry),
		flatAction(flatStateB, smf.ActionRun),
		flatAction(flatStateB, smf.ActionExit),
		flatAction(flatStateC, smf.ActionEntry),
		flatAction(flatStateC, smf.ActionRun),
		flatAction(flatStateC, smf.ActionExit),
	})

	// Two transitions completed before the third was aborted by terminate
	// in exit.
	rec.checkTransitions(t, name, []transitionRecord[*flatObject]{
		flatTransition(flatStateA, flatStateB),
		flatTransition(flatStateB, flatStateC),
	})

	rec.checkErrors(t, 0)
}
