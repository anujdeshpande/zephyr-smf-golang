// Copyright (c) 2024 Glenn Andrews
// State Machine example copyright (c) Miro Samek
// Copyright 2026 Anuj Deshpande
//
// Implementation of the statechart in Figure 2.11 of
// Practical UML Statecharts in C/C++, 2nd Edition by Miro Samek
// https://www.state-machine.com/psicc2
// Used with permission of the author.
//
// SPDX-License-Identifier: Apache-2.0

// Ported from samples/subsys/smf/hsm_psicc2/src/hsm_psicc2_thread.c.

package main

import (
	"fmt"
	"io"

	"github.com/anujdeshpande/zephyr-smf-golang/smf"
)

// Events that can be sent to the state machine. A to I correspond to events
// A through I in PSiCC2 Figure 2.11.
type event int

const (
	eventA event = iota
	eventB
	eventC
	eventD
	eventE
	eventF
	eventG
	eventH
	eventI
	eventTerminate
)

// object is the user defined object.
type object struct {
	smf.Ctx[*object]

	// Other state specific data.
	event event
	foo   bool
	log   io.Writer
}

func (o *object) logf(format string, args ...any) {
	fmt.Fprintf(o.log, format+"\n", args...)
}

// Possible states.
const (
	stateInitial = iota
	stateS
	stateS1
	stateS2
	stateS11
	stateS21
	stateS211
)

var stateNames = []string{"initial", "s", "s1", "s2", "s11", "s21", "s211"}

// The state table: handler functions, parent states and initial transition
// states. It is filled in by init, since the handlers refer to it.
var demoStates []smf.State[*object]

func init() {
	demoStates = make([]smf.State[*object], 7)
	demoStates[stateInitial] = smf.State[*object]{
		Entry: initialEntry, Run: initialRun, Exit: initialExit,
		Initial: &demoStates[stateS2],
	}
	demoStates[stateS] = smf.State[*object]{
		Entry: sEntry, Run: sRun, Exit: sExit,
		Parent: &demoStates[stateInitial], Initial: &demoStates[stateS11],
	}
	demoStates[stateS1] = smf.State[*object]{
		Entry: s1Entry, Run: s1Run, Exit: s1Exit,
		Parent: &demoStates[stateS], Initial: &demoStates[stateS11],
	}
	demoStates[stateS2] = smf.State[*object]{
		Entry: s2Entry, Run: s2Run, Exit: s2Exit,
		Parent: &demoStates[stateS], Initial: &demoStates[stateS211],
	}
	demoStates[stateS11] = smf.State[*object]{
		Entry: s11Entry, Run: s11Run, Exit: s11Exit,
		Parent: &demoStates[stateS1],
	}
	demoStates[stateS21] = smf.State[*object]{
		Entry: s21Entry, Run: s21Run, Exit: s21Exit,
		Parent: &demoStates[stateS2], Initial: &demoStates[stateS211],
	}
	demoStates[stateS211] = smf.State[*object]{
		Entry: s211Entry, Run: s211Run, Exit: s211Exit,
		Parent: &demoStates[stateS21],
	}
}

// STATE_INITIAL

func initialEntry(o *object) {
	o.logf("initial_entry")
	o.foo = false
}

func initialRun(o *object) smf.Result {
	o.logf("initial_run")
	return smf.Propagate
}

func initialExit(o *object) { o.logf("initial_exit") }

// STATE_S

func sEntry(o *object) { o.logf("s_entry") }

func sRun(o *object) smf.Result {
	o.logf("s_run")
	switch o.event {
	case eventE:
		o.logf("s_run received EVENT_E")
		o.SetState(&demoStates[stateS11])
	case eventI:
		if o.foo {
			o.logf("s_run received EVENT_I and set foo false")
			o.foo = false
		} else {
			o.logf("s_run received EVENT_I and did nothing")
		}
		return smf.Handled
	case eventTerminate:
		o.logf("s_run received SMF_EVENT_TERMINATE. Terminating")
		o.SetTerminate(-1)
	}
	return smf.Propagate
}

func sExit(o *object) { o.logf("s_exit") }

// STATE_S1

func s1Entry(o *object) { o.logf("s1_entry") }

func s1Run(o *object) smf.Result {
	o.logf("s1_run")
	switch o.event {
	case eventA:
		o.logf("s1_run received EVENT_A")
		o.SetState(&demoStates[stateS1])
	case eventB:
		o.logf("s1_run received EVENT_B")
		o.SetState(&demoStates[stateS11])
	case eventC:
		o.logf("s1_run received EVENT_C")
		o.SetState(&demoStates[stateS2])
	case eventD:
		if !o.foo {
			o.logf("s1_run received EVENT_D and acted on it")
			o.foo = true
			o.SetState(&demoStates[stateS])
		} else {
			o.logf("s1_run received EVENT_D and ignored it")
		}
	case eventF:
		o.logf("s1_run received EVENT_F")
		o.SetState(&demoStates[stateS211])
	case eventI:
		o.logf("s1_run received EVENT_I")
		return smf.Handled
	}
	return smf.Propagate
}

func s1Exit(o *object) { o.logf("s1_exit") }

// STATE_S11

func s11Entry(o *object) { o.logf("s11_entry") }

func s11Run(o *object) smf.Result {
	o.logf("s11_run")
	switch o.event {
	case eventD:
		if o.foo {
			o.logf("s11_run received EVENT_D and acted upon it")
			o.foo = false
			o.SetState(&demoStates[stateS1])
		} else {
			o.logf("s11_run received EVENT_D and ignored it")
		}
	case eventG:
		o.logf("s11_run received EVENT_G")
		o.SetState(&demoStates[stateS21])
	case eventH:
		o.logf("s11_run received EVENT_H")
		o.SetState(&demoStates[stateS])
	}
	return smf.Propagate
}

func s11Exit(o *object) { o.logf("s11_exit") }

// STATE_S2

func s2Entry(o *object) { o.logf("s2_entry") }

func s2Run(o *object) smf.Result {
	o.logf("s2_run")
	switch o.event {
	case eventC:
		o.logf("s2_run received EVENT_C")
		o.SetState(&demoStates[stateS1])
	case eventF:
		o.logf("s2_run received EVENT_F")
		o.SetState(&demoStates[stateS11])
	case eventI:
		if !o.foo {
			o.logf("s2_run received EVENT_I and set foo true")
			o.foo = true
			return smf.Handled
		}
		o.logf("s2_run received EVENT_I and did nothing")
	}
	return smf.Propagate
}

func s2Exit(o *object) { o.logf("s2_exit") }

// STATE_S21

func s21Entry(o *object) { o.logf("s21_entry") }

func s21Run(o *object) smf.Result {
	o.logf("s21_run")
	switch o.event {
	case eventA:
		o.logf("s21_run received EVENT_A")
		o.SetState(&demoStates[stateS21])
	case eventB:
		o.logf("s21_run received EVENT_B")
		o.SetState(&demoStates[stateS211])
	case eventG:
		o.logf("s21_run received EVENT_G")
		o.SetState(&demoStates[stateS1])
	}
	return smf.Propagate
}

func s21Exit(o *object) { o.logf("s21_exit") }

// STATE_S211

func s211Entry(o *object) { o.logf("s211_entry") }

func s211Run(o *object) smf.Result {
	o.logf("s211_run")
	switch o.event {
	case eventD:
		o.logf("s211_run received EVENT_D")
		o.SetState(&demoStates[stateS21])
	case eventH:
		o.logf("s211_run received EVENT_H")
		o.SetState(&demoStates[stateS])
	}
	return smf.Propagate
}

func s211Exit(o *object) { o.logf("s211_exit") }

// run starts the state machine and runs it once for each event received,
// until it terminates or events is closed. It is the hsm_psicc2_thread of
// the Zephyr sample, with a channel in place of the message queue.
func run(events <-chan event, log io.Writer) {
	obj := &object{log: log}
	obj.SetInitial(obj, &demoStates[stateInitial])
	for obj.event = range events {
		// The state machine terminates if a non-zero value is returned.
		if rc := obj.RunState(); rc != 0 {
			obj.logf("hsm_psicc2_thread terminating state machine thread")
			return
		}
	}
}
