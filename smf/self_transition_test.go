// Copyright 2024 Glenn Andrews
// based on test_lib_hierarchical_smf.c
// Copyright 2021 The Chromium OS Authors
// Copyright 2026 Anuj Deshpande
// SPDX-License-Identifier: Apache-2.0

// Ported from tests/lib/smf/src/test_lib_self_transition_smf.c.
//
// Hierarchical self-transition test:
//
// Topology: ROOT > {PARENT_AB > {A, B}, PARENT_C > {C}, D}
//
//	ROOT has initial = PARENT_AB
//	PARENT_AB has initial = STATE_A
//	PARENT_C has initial = STATE_C
//
// Normal flow:
//
//	SetInitial(PARENT_AB): follows initials to A, entry order ROOT,
//	PARENT_AB, A (not captured, hooks installed after init)
//
//	Run 0: A_RUN (transitions to B, sibling)
//	        -> A_EXIT, transition(A,B), B_ENTRY
//	Run 1: B_RUN (1st time, Handled, no propagation)
//	Run 2: B_RUN (2nd time, Propagate), PARENT_AB_RUN (transitions to STATE_C)
//	        -> B_EXIT, PARENT_AB_EXIT, transition(B,C), PARENT_C_ENTRY, C_ENTRY
//	Run 3: C_RUN (1st time, propagate), PARENT_C_RUN (self-transition to PARENT_C)
//	        -> C_EXIT, PARENT_C_EXIT(self), PARENT_C_ENTRY(self),
//	           transition(C,C), C_ENTRY
//	Run 4: C_RUN (2nd time, transitions to D)
//	        -> C_EXIT, PARENT_C_EXIT, transition(C,D), D_ENTRY

package smf_test

import (
	"testing"

	"github.com/anujdeshpande/zephyr-smf-golang/smf"
)

const selfSMFRun = 5

const (
	selfRoot = iota
	selfParentAB
	selfParentC
	selfStateA
	selfStateB
	selfStateC
	selfStateD
)

var selfNames = []string{"ROOT", "PARENT_AB", "PARENT_C", "STATE_A", "STATE_B", "STATE_C", "STATE_D"}

type selfTerminateAction int

const (
	selfNone selfTerminateAction = iota
	selfParentEntry
	selfParentRun
	selfParentExit
	selfEntry
	selfRun
	selfExit
)

type selfObject struct {
	smf.Ctx[*selfObject]
	terminate selfTerminateAction
	bRunCount int
	cRunCount int
}

var selfStates []smf.State[*selfObject]

func init() {
	selfStates = make([]smf.State[*selfObject], 7)
	selfStates[selfRoot] = smf.State[*selfObject]{
		Entry: selfRootEntry, Run: selfRootRun, Exit: selfRootExit,
		Initial: &selfStates[selfParentAB],
	}
	selfStates[selfParentAB] = smf.State[*selfObject]{
		Entry: selfParentABEntry, Run: selfParentABRun, Exit: selfParentABExit,
		Parent: &selfStates[selfRoot], Initial: &selfStates[selfStateA],
	}
	selfStates[selfParentC] = smf.State[*selfObject]{
		Entry: selfParentCEntry, Run: selfParentCRun, Exit: selfParentCExit,
		Parent: &selfStates[selfRoot], Initial: &selfStates[selfStateC],
	}
	selfStates[selfStateA] = smf.State[*selfObject]{
		Entry: selfStateAEntry, Run: selfStateARun, Exit: selfStateAExit,
		Parent: &selfStates[selfParentAB],
	}
	selfStates[selfStateB] = smf.State[*selfObject]{
		Entry: selfStateBEntry, Run: selfStateBRun, Exit: selfStateBExit,
		Parent: &selfStates[selfParentAB],
	}
	selfStates[selfStateC] = smf.State[*selfObject]{
		Entry: selfStateCEntry, Run: selfStateCRun, Exit: selfStateCExit,
		Parent: &selfStates[selfParentC],
	}
	selfStates[selfStateD] = smf.State[*selfObject]{
		Entry: selfStateDEntry, Run: selfStateDRun, Exit: selfStateDExit,
		Parent: &selfStates[selfRoot],
	}
}

// ROOT handlers

func selfRootEntry(*selfObject)          {}
func selfRootRun(*selfObject) smf.Result { return smf.Propagate }
func selfRootExit(*selfObject)           {}

// PARENT_AB handlers

func selfParentABEntry(o *selfObject) {
	if o.terminate == selfParentEntry {
		o.SetTerminate(-1)
	}
}

func selfParentABRun(o *selfObject) smf.Result {
	if o.terminate == selfParentRun {
		o.SetTerminate(-1)
		return smf.Propagate
	}

	o.SetState(&selfStates[selfStateC])
	return smf.Handled
}

func selfParentABExit(o *selfObject) {
	if o.terminate == selfParentExit {
		o.SetTerminate(-1)
	}
}

// PARENT_C handlers

func selfParentCEntry(*selfObject) {}

func selfParentCRun(o *selfObject) smf.Result {
	// Self-transition to PARENT_C.
	o.SetState(&selfStates[selfParentC])
	return smf.Propagate
}

func selfParentCExit(*selfObject) {}

// STATE_A handlers

func selfStateAEntry(o *selfObject) {
	if o.terminate == selfEntry {
		o.SetTerminate(-1)
	}
}

func selfStateARun(o *selfObject) smf.Result {
	o.SetState(&selfStates[selfStateB])
	return smf.Propagate
}

func selfStateAExit(*selfObject) {}

// STATE_B handlers

func selfStateBEntry(*selfObject) {}

func selfStateBRun(o *selfObject) smf.Result {
	if o.terminate == selfRun {
		o.SetTerminate(-1)
		return smf.Propagate
	}

	o.bRunCount++
	if o.bRunCount == 1 {
		// First time: handle event, suppress propagation.
		return smf.Handled
	}
	// Second time: propagate to PARENT_AB.
	return smf.Propagate
}

func selfStateBExit(*selfObject) {}

// STATE_C handlers

func selfStateCEntry(*selfObject) {}

func selfStateCRun(o *selfObject) smf.Result {
	o.cRunCount++
	if o.cRunCount == 1 {
		// First time: propagate to parent (PARENT_C does self-transition).
		return smf.Propagate
	}
	// Second time: transition to D.
	o.SetState(&selfStates[selfStateD])
	return smf.Propagate
}

func selfStateCExit(o *selfObject) {
	if o.terminate == selfExit {
		o.SetTerminate(-1)
	}
}

// STATE_D handlers

func selfStateDEntry(*selfObject)          {}
func selfStateDRun(*selfObject) smf.Result { return smf.Propagate }
func selfStateDExit(*selfObject)           {}

func runSelfFSM(term selfTerminateAction) (*selfObject, *recorder[*selfObject]) {
	rec := &recorder[*selfObject]{}
	obj := &selfObject{terminate: term}
	obj.SetInitial(obj, &selfStates[selfParentAB])
	obj.SetHooks(rec.hooks())

	for range selfSMFRun {
		if obj.RunState() < 0 {
			break
		}
	}
	return obj, rec
}

func selfAction(state int, action smf.Action) actionRecord[*selfObject] {
	return actionRecord[*selfObject]{&selfStates[state], action}
}

func selfTransition(source, dest int) transitionRecord[*selfObject] {
	return transitionRecord[*selfObject]{&selfStates[source], &selfStates[dest]}
}

func TestSMFSelfTransitionInstrumentedNormal(t *testing.T) {
	obj, rec := runSelfFSM(selfNone)
	name := indexName(selfStates, selfNames)

	rec.checkActions(t, name, []actionRecord[*selfObject]{
		// Initial entry ROOT, PARENT_AB, A not captured

		// Run 0: A_RUN transitions to B (sibling under PARENT_AB)
		selfAction(selfStateA, smf.ActionRun),
		selfAction(selfStateA, smf.ActionExit),
		selfAction(selfStateB, smf.ActionEntry),

		// Run 1: B_RUN (1st time, Handled: no propagation, no transition)
		selfAction(selfStateB, smf.ActionRun),

		// Run 2: B_RUN (2nd, Propagate) -> PARENT_AB_RUN (transitions to C)
		selfAction(selfStateB, smf.ActionRun),
		selfAction(selfParentAB, smf.ActionRun),
		selfAction(selfStateB, smf.ActionExit),
		selfAction(selfParentAB, smf.ActionExit),
		selfAction(selfParentC, smf.ActionEntry),
		selfAction(selfStateC, smf.ActionEntry),

		// Run 3: C_RUN (1st, Propagate) -> PARENT_C_RUN (self-transition)
		selfAction(selfStateC, smf.ActionRun),
		selfAction(selfParentC, smf.ActionRun),
		selfAction(selfStateC, smf.ActionExit),
		selfAction(selfParentC, smf.ActionExit),
		selfAction(selfParentC, smf.ActionEntry),
		selfAction(selfStateC, smf.ActionEntry),

		// Run 4: C_RUN (2nd, transitions to D)
		selfAction(selfStateC, smf.ActionRun),
		selfAction(selfStateC, smf.ActionExit),
		selfAction(selfParentC, smf.ActionExit),
		selfAction(selfStateD, smf.ActionEntry),
	})

	// Transitions: A->B, B->C, C->C (self via parent), C->D
	rec.checkTransitions(t, name, []transitionRecord[*selfObject]{
		selfTransition(selfStateA, selfStateB),
		selfTransition(selfStateB, selfStateC),
		// Self-transition: previous was C, and after following
		// PARENT_C.Initial, current is C again.
		selfTransition(selfStateC, selfStateC),
		selfTransition(selfStateC, selfStateD),
	})

	if got := obj.Current(); got != &selfStates[selfStateD] {
		t.Errorf("final state %s, want STATE_D", name(got))
	}

	rec.checkErrors(t, 0)
}

// Terminate in parent entry: terminates during SetInitial, hooks not
// installed yet. No actions or transitions captured.
func TestSMFSelfTransitionInstrumentedParentEntryTerminate(t *testing.T) {
	_, rec := runSelfFSM(selfParentEntry)
	name := indexName(selfStates, selfNames)

	rec.checkActions(t, name, nil)
	rec.checkTransitions(t, name, nil)
	rec.checkErrors(t, 0)
}

// Terminate in child entry: terminates during SetInitial, hooks not
// installed yet. No actions or transitions captured.
func TestSMFSelfTransitionInstrumentedChildEntryTerminate(t *testing.T) {
	_, rec := runSelfFSM(selfEntry)
	name := indexName(selfStates, selfNames)

	rec.checkActions(t, name, nil)
	rec.checkTransitions(t, name, nil)
	rec.checkErrors(t, 0)
}

// Terminate in parent run (PARENT_AB_RUN) (initial entries not captured).
func TestSMFSelfTransitionInstrumentedParentRunTerminate(t *testing.T) {
	_, rec := runSelfFSM(selfParentRun)
	name := indexName(selfStates, selfNames)

	rec.checkActions(t, name, []actionRecord[*selfObject]{
		selfAction(selfStateA, smf.ActionRun),
		selfAction(selfStateA, smf.ActionExit),
		selfAction(selfStateB, smf.ActionEntry),
		selfAction(selfStateB, smf.ActionRun),
		selfAction(selfStateB, smf.ActionRun),
		selfAction(selfParentAB, smf.ActionRun),
	})

	rec.checkTransitions(t, name, []transitionRecord[*selfObject]{
		selfTransition(selfStateA, selfStateB),
	})

	rec.checkErrors(t, 0)
}

// Terminate in parent exit (PARENT_AB_EXIT) (initial entries not captured).
func TestSMFSelfTransitionInstrumentedParentExitTerminate(t *testing.T) {
	_, rec := runSelfFSM(selfParentExit)
	name := indexName(selfStates, selfNames)

	rec.checkActions(t, name, []actionRecord[*selfObject]{
		selfAction(selfStateA, smf.ActionRun),
		selfAction(selfStateA, smf.ActionExit),
		selfAction(selfStateB, smf.ActionEntry),
		selfAction(selfStateB, smf.ActionRun),
		selfAction(selfStateB, smf.ActionRun),
		selfAction(selfParentAB, smf.ActionRun),
		selfAction(selfStateB, smf.ActionExit),
		selfAction(selfParentAB, smf.ActionExit),
	})

	rec.checkTransitions(t, name, []transitionRecord[*selfObject]{
		selfTransition(selfStateA, selfStateB),
	})

	rec.checkErrors(t, 0)
}

// Terminate in child run (B_RUN) (initial entries not captured).
func TestSMFSelfTransitionInstrumentedChildRunTerminate(t *testing.T) {
	_, rec := runSelfFSM(selfRun)
	name := indexName(selfStates, selfNames)

	rec.checkActions(t, name, []actionRecord[*selfObject]{
		selfAction(selfStateA, smf.ActionRun),
		selfAction(selfStateA, smf.ActionExit),
		selfAction(selfStateB, smf.ActionEntry),
		selfAction(selfStateB, smf.ActionRun),
	})

	rec.checkTransitions(t, name, []transitionRecord[*selfObject]{
		selfTransition(selfStateA, selfStateB),
	})

	rec.checkErrors(t, 0)
}

// Terminate in child exit (C_EXIT) (initial entries not captured).
func TestSMFSelfTransitionInstrumentedChildExitTerminate(t *testing.T) {
	_, rec := runSelfFSM(selfExit)
	name := indexName(selfStates, selfNames)

	rec.checkActions(t, name, []actionRecord[*selfObject]{
		// Run 0
		selfAction(selfStateA, smf.ActionRun),
		selfAction(selfStateA, smf.ActionExit),
		selfAction(selfStateB, smf.ActionEntry),
		// Run 1
		selfAction(selfStateB, smf.ActionRun),
		// Run 2
		selfAction(selfStateB, smf.ActionRun),
		selfAction(selfParentAB, smf.ActionRun),
		selfAction(selfStateB, smf.ActionExit),
		selfAction(selfParentAB, smf.ActionExit),
		selfAction(selfParentC, smf.ActionEntry),
		selfAction(selfStateC, smf.ActionEntry),
		// Run 3: C_RUN propagates, PARENT_C_RUN does self-transition,
		// C_EXIT terminates
		selfAction(selfStateC, smf.ActionRun),
		selfAction(selfParentC, smf.ActionRun),
		selfAction(selfStateC, smf.ActionExit),
	})

	// Two transitions completed, self-transition aborted during exit.
	rec.checkTransitions(t, name, []transitionRecord[*selfObject]{
		selfTransition(selfStateA, selfStateB),
		selfTransition(selfStateB, selfStateC),
	})

	rec.checkErrors(t, 0)
}
