// Copyright 2021 The Chromium OS Authors
// Copyright 2026 Anuj Deshpande
// SPDX-License-Identifier: Apache-2.0

// Ported from tests/lib/smf/src/test_lib_hierarchical_smf.c.
//
// Hierarchical Test Transition:
//
// Hierarchy: PARENT_AB{A, B}, PARENT_C{C}, D (all parents are roots)
//
// Normal flow after SetInitial(A) (PARENT_AB_ENTRY, A_ENTRY during init,
// not captured, hooks installed after init):
//
//	Run 0: A_RUN (propagate) --> PARENT_AB_RUN (transitions to B)
//	        --> A_EXIT, transition(A,B), B_ENTRY
//	Run 1: B_RUN (transitions to C)
//	        --> B_EXIT, PARENT_AB_EXIT, transition(B,C), PARENT_C_ENTRY, C_ENTRY
//	Run 2: C_RUN (transitions to D, returns Handled so PARENT_C_RUN not called)
//	        --> C_EXIT, PARENT_C_EXIT, transition(C,D), D_ENTRY

package smf_test

import (
	"testing"

	"github.com/anujdeshpande/zephyr-smf-golang/smf"
)

const hsmSMFRun = 3

const (
	hsmParentAB = iota
	hsmParentC
	hsmStateA
	hsmStateB
	hsmStateC
	hsmStateD
)

var hsmNames = []string{"PARENT_AB", "PARENT_C", "STATE_A", "STATE_B", "STATE_C", "STATE_D"}

type hsmTerminateAction int

const (
	hsmNone hsmTerminateAction = iota
	hsmParentEntry
	hsmParentRun
	hsmParentExit
	hsmEntry
	hsmRun
	hsmExit
)

type hsmObject struct {
	smf.Ctx[*hsmObject]
	terminate hsmTerminateAction
}

var hsmStates []smf.State[*hsmObject]

func init() {
	hsmStates = make([]smf.State[*hsmObject], 6)
	hsmStates[hsmParentAB] = smf.State[*hsmObject]{
		Entry: hsmParentABEntry, Run: hsmParentABRun, Exit: hsmParentABExit,
	}
	hsmStates[hsmParentC] = smf.State[*hsmObject]{
		Entry: hsmParentCEntry, Run: hsmParentCRun, Exit: hsmParentCExit,
	}
	hsmStates[hsmStateA] = smf.State[*hsmObject]{
		Entry: hsmStateAEntry, Run: hsmStateARun, Exit: hsmStateAExit,
		Parent: &hsmStates[hsmParentAB],
	}
	hsmStates[hsmStateB] = smf.State[*hsmObject]{
		Entry: hsmStateBEntry, Run: hsmStateBRun, Exit: hsmStateBExit,
		Parent: &hsmStates[hsmParentAB],
	}
	hsmStates[hsmStateC] = smf.State[*hsmObject]{
		Entry: hsmStateCEntry, Run: hsmStateCRun, Exit: hsmStateCExit,
		Parent: &hsmStates[hsmParentC],
	}
	hsmStates[hsmStateD] = smf.State[*hsmObject]{
		Entry: hsmStateDEntry, Run: hsmStateDRun, Exit: hsmStateDExit,
	}
}

// Parent AB handlers

func hsmParentABEntry(o *hsmObject) {
	if o.terminate == hsmParentEntry {
		o.SetTerminate(-1)
	}
}

func hsmParentABRun(o *hsmObject) smf.Result {
	if o.terminate == hsmParentRun {
		o.SetTerminate(-1)
		return smf.Propagate
	}

	o.SetState(&hsmStates[hsmStateB])
	return smf.Propagate
}

func hsmParentABExit(o *hsmObject) {
	if o.terminate == hsmParentExit {
		o.SetTerminate(-1)
	}
}

// Parent C handlers

func hsmParentCEntry(*hsmObject) {}

func hsmParentCRun(*hsmObject) smf.Result {
	// Should not be reached in the normal flow since C handles the event.
	return smf.Propagate
}

func hsmParentCExit(*hsmObject) {}

// State A handlers

func hsmStateAEntry(o *hsmObject) {
	if o.terminate == hsmEntry {
		o.SetTerminate(-1)
	}
}

func hsmStateARun(*hsmObject) smf.Result {
	// Propagate to parent (PARENT_AB_RUN will handle).
	return smf.Propagate
}

func hsmStateAExit(*hsmObject) {}

// State B handlers

func hsmStateBEntry(*hsmObject) {}

func hsmStateBRun(o *hsmObject) smf.Result {
	if o.terminate == hsmRun {
		o.SetTerminate(-1)
		return smf.Propagate
	}

	o.SetState(&hsmStates[hsmStateC])
	return smf.Propagate
}

func hsmStateBExit(*hsmObject) {}

// State C handlers

func hsmStateCEntry(*hsmObject) {}

func hsmStateCRun(o *hsmObject) smf.Result {
	o.SetState(&hsmStates[hsmStateD])
	return smf.Handled
}

func hsmStateCExit(o *hsmObject) {
	if o.terminate == hsmExit {
		o.SetTerminate(-1)
	}
}

// State D handlers

func hsmStateDEntry(*hsmObject) {}

func hsmStateDRun(*hsmObject) smf.Result { return smf.Propagate }

func hsmStateDExit(*hsmObject) {}

func runHSMFSM(initial int, term hsmTerminateAction) (*hsmObject, *recorder[*hsmObject]) {
	rec := &recorder[*hsmObject]{}
	obj := &hsmObject{terminate: term}
	obj.SetInitial(obj, &hsmStates[initial])
	obj.SetHooks(rec.hooks())

	for range hsmSMFRun {
		if obj.RunState() < 0 {
			break
		}
	}
	return obj, rec
}

func hsmAction(state int, action smf.Action) actionRecord[*hsmObject] {
	return actionRecord[*hsmObject]{&hsmStates[state], action}
}

func hsmTransition(source, dest int) transitionRecord[*hsmObject] {
	return transitionRecord[*hsmObject]{&hsmStates[source], &hsmStates[dest]}
}

// Normal hierarchical transitions: init(A), A propagates to PARENT_AB which
// transitions to B, B transitions to C (cross-parent), C transitions to D.
//
// Initial PARENT_AB_ENTRY, A_ENTRY not captured (hooks installed after init).
func TestSMFHierarchicalInstrumentedNormal(t *testing.T) {
	obj, rec := runHSMFSM(hsmStateA, hsmNone)
	name := indexName(hsmStates, hsmNames)

	// Expected action sequence (initial entries not captured).
	rec.checkActions(t, name, []actionRecord[*hsmObject]{
		// Run 0
		hsmAction(hsmStateA, smf.ActionRun),
		hsmAction(hsmParentAB, smf.ActionRun),
		hsmAction(hsmStateA, smf.ActionExit),
		hsmAction(hsmStateB, smf.ActionEntry),
		// Run 1
		hsmAction(hsmStateB, smf.ActionRun),
		hsmAction(hsmStateB, smf.ActionExit),
		hsmAction(hsmParentAB, smf.ActionExit),
		hsmAction(hsmParentC, smf.ActionEntry),
		hsmAction(hsmStateC, smf.ActionEntry),
		// Run 2
		hsmAction(hsmStateC, smf.ActionRun),
		hsmAction(hsmStateC, smf.ActionExit),
		hsmAction(hsmParentC, smf.ActionExit),
		hsmAction(hsmStateD, smf.ActionEntry),
	})

	// Verify transitions.
	rec.checkTransitions(t, name, []transitionRecord[*hsmObject]{
		hsmTransition(hsmStateA, hsmStateB),
		hsmTransition(hsmStateB, hsmStateC),
		hsmTransition(hsmStateC, hsmStateD),
	})

	if got := obj.Current(); got != &hsmStates[hsmStateD] {
		t.Errorf("final state %s, want STATE_D", name(got))
	}

	rec.checkErrors(t, 0)
}

// Terminate in parent entry: terminates during SetInitial, hooks not
// installed yet. No actions or transitions captured.
func TestSMFHierarchicalInstrumentedParentEntryTerminate(t *testing.T) {
	_, rec := runHSMFSM(hsmStateA, hsmParentEntry)
	name := indexName(hsmStates, hsmNames)

	rec.checkActions(t, name, nil)
	rec.checkTransitions(t, name, nil)
	rec.checkErrors(t, 0)
}

// Terminate in child entry: terminates during SetInitial, hooks not
// installed yet. No actions or transitions captured.
func TestSMFHierarchicalInstrumentedChildEntryTerminate(t *testing.T) {
	_, rec := runHSMFSM(hsmStateA, hsmEntry)
	name := indexName(hsmStates, hsmNames)

	rec.checkActions(t, name, nil)
	rec.checkTransitions(t, name, nil)
	rec.checkErrors(t, 0)
}

// Terminate in parent run: A_RUN propagates, PARENT_AB_RUN terminates
// (initial entries not captured).
func TestSMFHierarchicalInstrumentedParentRunTerminate(t *testing.T) {
	_, rec := runHSMFSM(hsmStateA, hsmParentRun)
	name := indexName(hsmStates, hsmNames)

	rec.checkActions(t, name, []actionRecord[*hsmObject]{
		hsmAction(hsmStateA, smf.ActionRun),
		hsmAction(hsmParentAB, smf.ActionRun),
	})
	rec.checkTransitions(t, name, nil)
	rec.checkErrors(t, 0)
}

// Terminate in parent exit (initial entries not captured).
func TestSMFHierarchicalInstrumentedParentExitTerminate(t *testing.T) {
	_, rec := runHSMFSM(hsmStateA, hsmParentExit)
	name := indexName(hsmStates, hsmNames)

	rec.checkActions(t, name, []actionRecord[*hsmObject]{
		hsmAction(hsmStateA, smf.ActionRun),
		hsmAction(hsmParentAB, smf.ActionRun),
		hsmAction(hsmStateA, smf.ActionExit),
		hsmAction(hsmStateB, smf.ActionEntry),
		hsmAction(hsmStateB, smf.ActionRun),
		hsmAction(hsmStateB, smf.ActionExit),
		hsmAction(hsmParentAB, smf.ActionExit),
	})

	// First transition (A->B) completed, second aborted during exit.
	rec.checkTransitions(t, name, []transitionRecord[*hsmObject]{
		hsmTransition(hsmStateA, hsmStateB),
	})

	rec.checkErrors(t, 0)
}

// Terminate in child run (B_RUN) (initial entries not captured).
func TestSMFHierarchicalInstrumentedChildRunTerminate(t *testing.T) {
	_, rec := runHSMFSM(hsmStateA, hsmRun)
	name := indexName(hsmStates, hsmNames)

	rec.checkActions(t, name, []actionRecord[*hsmObject]{
		hsmAction(hsmStateA, smf.ActionRun),
		hsmAction(hsmParentAB, smf.ActionRun),
		hsmAction(hsmStateA, smf.ActionExit),
		hsmAction(hsmStateB, smf.ActionEntry),
		hsmAction(hsmStateB, smf.ActionRun),
	})

	rec.checkTransitions(t, name, []transitionRecord[*hsmObject]{
		hsmTransition(hsmStateA, hsmStateB),
	})

	rec.checkErrors(t, 0)
}

// Terminate in child exit (C_EXIT) (initial entries not captured).
func TestSMFHierarchicalInstrumentedChildExitTerminate(t *testing.T) {
	_, rec := runHSMFSM(hsmStateA, hsmExit)
	name := indexName(hsmStates, hsmNames)

	rec.checkActions(t, name, []actionRecord[*hsmObject]{
		// Run 0
		hsmAction(hsmStateA, smf.ActionRun),
		hsmAction(hsmParentAB, smf.ActionRun),
		hsmAction(hsmStateA, smf.ActionExit),
		hsmAction(hsmStateB, smf.ActionEntry),
		// Run 1
		hsmAction(hsmStateB, smf.ActionRun),
		hsmAction(hsmStateB, smf.ActionExit),
		hsmAction(hsmParentAB, smf.ActionExit),
		hsmAction(hsmParentC, smf.ActionEntry),
		hsmAction(hsmStateC, smf.ActionEntry),
		// Run 2
		hsmAction(hsmStateC, smf.ActionRun),
		hsmAction(hsmStateC, smf.ActionExit),
	})

	// Two transitions completed, third aborted.
	rec.checkTransitions(t, name, []transitionRecord[*hsmObject]{
		hsmTransition(hsmStateA, hsmStateB),
		hsmTransition(hsmStateB, hsmStateC),
	})

	rec.checkErrors(t, 0)
}
