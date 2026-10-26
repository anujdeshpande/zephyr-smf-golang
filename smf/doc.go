// Copyright 2021 The Chromium OS Authors
// Copyright 2026 Anuj Deshpande
// SPDX-License-Identifier: Apache-2.0

// Package smf is a Go port of the Zephyr RTOS State Machine Framework (SMF),
// from Zephyr v4.4.2 (SMF API version 0.2.0).
//
// It runs flat and hierarchical state machines with the same semantics as
// Zephyr's lib/smf/smf.c, including ancestor states, initial transitions and
// instrumentation hooks. In Zephyr these are Kconfig options; here they are
// always available and cost nothing when unused. A state without a Parent
// behaves exactly as it does in a flat Zephyr state machine.
//
// # States
//
// A [State] has optional Entry, Run and Exit actions, an optional Parent and,
// for a parent state, an optional Initial child. Actions receive the user
// defined object, which embeds a [Ctx] in place of Zephyr's struct smf_ctx:
//
//	type object struct {
//		smf.Ctx[*object]
//		// User defined data follows.
//	}
//
// Run returns [Handled] when the state dealt with the current event, or
// [Propagate] to pass it to the parent's Run. Calling [Ctx.SetState] from Run
// also stops propagation.
//
// # Running a state machine
//
// [Ctx.SetInitial] enters the initial state. [Ctx.RunState] runs one
// iteration: the current state's Run, then its ancestors' Run actions until
// one handles the event or changes state. An application stops calling
// RunState once it returns a non-zero value, which is the value passed to
// [Ctx.SetTerminate].
//
// # Transitions
//
// SMF follows UML hierarchical state machine rules: the entry and exit
// actions of the least common ancestor of the source and target do not run,
// unless the transition is to self. Like Zephyr, it differs from UML in that
// it runs transition actions in the context of the source state, allows only
// external transitions to self, and refuses [Ctx.SetState] in exit actions.
//
// # Concurrency
//
// A Ctx is not safe for concurrent use. Feed events to a state machine from
// one goroutine, for example by reading them from a channel.
package smf
