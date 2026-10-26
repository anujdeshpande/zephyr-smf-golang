// Copyright 2021 The Chromium OS Authors
// Copyright 2026 Anuj Deshpande
// SPDX-License-Identifier: Apache-2.0

// Ported from tests/lib/smf/src/test_lib_hierarchical_5_ancestor_smf.c.
//
// Hierarchical 5-ancestor test:
//
// Hierarchy: P05 > P04 > P03 > P02 > P01 > {A, B}, C (root), D (root)
//
// SetInitial(A): P05_ENTRY .. P01_ENTRY, A_ENTRY (not captured, hooks
// installed after init)
//
//	Run 0: A_RUN (transitions to B, sibling under P01)
//	        -> A_EXIT, transition(A,B), B_ENTRY
//	Run 1: B_RUN (propagate), P01_RUN..P05_RUN (P05 transitions to C)
//	        -> B_EXIT, P01_EXIT..P05_EXIT, transition(B,C), C_ENTRY
//	Run 2: C_RUN (transitions to D)
//	        -> C_EXIT, transition(C,D), D_ENTRY

package smf_test

import (
	"testing"

	"github.com/anujdeshpande/zephyr-smf-golang/smf"
)

const h5SMFRun = 3

const (
	h5P05 = iota
	h5P04
	h5P03
	h5P02
	h5P01
	h5A
	h5B
	h5C
	h5D
)

var h5Names = []string{"P05", "P04", "P03", "P02", "P01", "A", "B", "C", "D"}

type h5Object struct {
	smf.Ctx[*h5Object]
}

var h5States []smf.State[*h5Object]

func init() {
	h5States = make([]smf.State[*h5Object], 9)
	h5States[h5P05] = smf.State[*h5Object]{Entry: h5P05Entry, Run: h5P05Run, Exit: h5P05Exit}
	h5States[h5P04] = smf.State[*h5Object]{Entry: h5P04Entry, Run: h5P04Run, Exit: h5P04Exit, Parent: &h5States[h5P05]}
	h5States[h5P03] = smf.State[*h5Object]{Entry: h5P03Entry, Run: h5P03Run, Exit: h5P03Exit, Parent: &h5States[h5P04]}
	h5States[h5P02] = smf.State[*h5Object]{Entry: h5P02Entry, Run: h5P02Run, Exit: h5P02Exit, Parent: &h5States[h5P03]}
	h5States[h5P01] = smf.State[*h5Object]{Entry: h5P01Entry, Run: h5P01Run, Exit: h5P01Exit, Parent: &h5States[h5P02]}
	h5States[h5A] = smf.State[*h5Object]{Entry: h5AEntry, Run: h5ARun, Exit: h5AExit, Parent: &h5States[h5P01]}
	h5States[h5B] = smf.State[*h5Object]{Entry: h5BEntry, Run: h5BRun, Exit: h5BExit, Parent: &h5States[h5P01]}
	h5States[h5C] = smf.State[*h5Object]{Entry: h5CEntry, Run: h5CRun, Exit: h5CExit}
	h5States[h5D] = smf.State[*h5Object]{Entry: h5DEntry}
}

// Parent handlers: all propagate, P05 transitions to C.

func h5P05Entry(*h5Object) {}
func h5P05Run(o *h5Object) smf.Result {
	o.SetState(&h5States[h5C])
	return smf.Propagate
}
func h5P05Exit(*h5Object) {}

func h5P04Entry(*h5Object)          {}
func h5P04Run(*h5Object) smf.Result { return smf.Propagate }
func h5P04Exit(*h5Object)           {}
func h5P03Entry(*h5Object)          {}
func h5P03Run(*h5Object) smf.Result { return smf.Propagate }
func h5P03Exit(*h5Object)           {}
func h5P02Entry(*h5Object)          {}
func h5P02Run(*h5Object) smf.Result { return smf.Propagate }
func h5P02Exit(*h5Object)           {}
func h5P01Entry(*h5Object)          {}
func h5P01Run(*h5Object) smf.Result { return smf.Propagate }
func h5P01Exit(*h5Object)           {}

// Leaf state A: transitions to sibling B.

func h5AEntry(*h5Object) {}
func h5ARun(o *h5Object) smf.Result {
	o.SetState(&h5States[h5B])
	return smf.Propagate
}
func h5AExit(*h5Object) {}

// Leaf state B: propagates (ancestors handle).

func h5BEntry(*h5Object)          {}
func h5BRun(*h5Object) smf.Result { return smf.Propagate }
func h5BExit(*h5Object)           {}

// Root state C: transitions to D.

func h5CEntry(*h5Object) {}
func h5CRun(o *h5Object) smf.Result {
	o.SetState(&h5States[h5D])
	return smf.Propagate
}
func h5CExit(*h5Object) {}

// Root state D: terminal, has entry only.

func h5DEntry(*h5Object) {}

func h5Action(state int, action smf.Action) actionRecord[*h5Object] {
	return actionRecord[*h5Object]{&h5States[state], action}
}

func h5Transition(source, dest int) transitionRecord[*h5Object] {
	return transitionRecord[*h5Object]{&h5States[source], &h5States[dest]}
}

func TestSMFHierarchical5AncestorsInstrumented(t *testing.T) {
	rec := &recorder[*h5Object]{}
	obj := &h5Object{}
	obj.SetInitial(obj, &h5States[h5A])
	obj.SetHooks(rec.hooks())

	for range h5SMFRun {
		if obj.RunState() < 0 {
			break
		}
	}

	name := indexName(h5States, h5Names)

	// Expected action trace (initial entries not captured):
	//
	// Run 0: A_RUN (transitions to B, sibling under P01)
	//   A_RUN, A_EXIT, B_ENTRY
	//
	// Run 1: B_RUN (propagate), then ancestor runs bubble up,
	//   P05_RUN transitions to C
	//   B_RUN, P01_RUN, P02_RUN, P03_RUN, P04_RUN, P05_RUN
	//   then exits: B_EXIT, P01_EXIT, P02_EXIT, P03_EXIT, P04_EXIT, P05_EXIT
	//   then entry: C_ENTRY
	//
	// Run 2: C_RUN (transitions to D)
	//   C_RUN, C_EXIT, D_ENTRY
	rec.checkActions(t, name, []actionRecord[*h5Object]{
		// Run 0
		h5Action(h5A, smf.ActionRun),
		h5Action(h5A, smf.ActionExit),
		h5Action(h5B, smf.ActionEntry),
		// Run 1: leaf + ancestor runs
		h5Action(h5B, smf.ActionRun),
		h5Action(h5P01, smf.ActionRun),
		h5Action(h5P02, smf.ActionRun),
		h5Action(h5P03, smf.ActionRun),
		h5Action(h5P04, smf.ActionRun),
		h5Action(h5P05, smf.ActionRun),
		// Run 1: exit chain (current=B, up to nil)
		h5Action(h5B, smf.ActionExit),
		h5Action(h5P01, smf.ActionExit),
		h5Action(h5P02, smf.ActionExit),
		h5Action(h5P03, smf.ActionExit),
		h5Action(h5P04, smf.ActionExit),
		h5Action(h5P05, smf.ActionExit),
		// Run 1: entry into C
		h5Action(h5C, smf.ActionEntry),
		// Run 2
		h5Action(h5C, smf.ActionRun),
		h5Action(h5C, smf.ActionExit),
		h5Action(h5D, smf.ActionEntry),
	})

	// Verify transitions.
	rec.checkTransitions(t, name, []transitionRecord[*h5Object]{
		h5Transition(h5A, h5B),
		h5Transition(h5B, h5C),
		h5Transition(h5C, h5D),
	})

	if got := obj.Current(); got != &h5States[h5D] {
		t.Errorf("final state %s, want D", name(got))
	}

	rec.checkErrors(t, 0)
}
