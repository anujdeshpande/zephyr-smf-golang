// Copyright 2021 The Chromium OS Authors
// Copyright 2026 Anuj Deshpande
// SPDX-License-Identifier: Apache-2.0

package smf

import (
	"errors"
	"strconv"
)

// Result is the value a Run action returns: whether the event was handled
// or should propagate to the parent state's Run action. It corresponds to
// Zephyr's enum smf_state_result.
type Result int

const (
	Handled   Result = iota // The state handled the event (SMF_EVENT_HANDLED).
	Propagate               // Pass the event to the parent state (SMF_EVENT_PROPAGATE).
)

// Action identifies which action of a state is executing. It is passed to
// [Hooks.OnAction] and corresponds to Zephyr's enum smf_action_type.
type Action int

const (
	ActionEntry Action = iota // Entry action (SMF_ACTION_ENTRY).
	ActionRun                 // Run action (SMF_ACTION_RUN).
	ActionExit                // Exit action (SMF_ACTION_EXIT).
)

func (a Action) String() string {
	switch a {
	case ActionEntry:
		return "entry"
	case ActionRun:
		return "run"
	case ActionExit:
		return "exit"
	}
	return "Action(" + strconv.Itoa(int(a)) + ")"
}

// Errors reported to [Hooks.OnError]. In both cases no transition occurs.
var (
	// ErrNilTransition is reported when SetState is called with a nil
	// state (SMF_ERR_NULL_TRANSITION).
	ErrNilTransition = errors.New("smf: transition to a nil state")
	// ErrTransitionInExit is reported when SetState is called from an
	// exit action (SMF_ERR_TRANSITION_IN_EXIT).
	ErrTransitionInExit = errors.New("smf: SetState called from an exit action")
)

// State is a state of a state machine operating on objects of type T.
// It corresponds to Zephyr's struct smf_state and SMF_CREATE_STATE.
//
// All fields are optional. States are compared by address, so a state
// machine refers to its states through pointers that stay valid while it
// runs, typically into a package-level array indexed by a state enum.
type State[T any] struct {
	// Entry runs when the state is entered. A parent's Entry runs before
	// its child's.
	Entry func(obj T)
	// Run runs on each RunState iteration. A parent's Run runs after its
	// child's, and only if the child returned Propagate without calling
	// SetState.
	Run func(obj T) Result
	// Exit runs when the state is left. A parent's Exit runs after its
	// child's.
	Exit func(obj T)

	// Parent holds entry, run and exit actions common to its children.
	// A transition between two children of a parent runs neither the
	// parent's Exit nor its Entry.
	Parent *State[T]
	// Initial is the child that entering this state transitions to. It is
	// nil for leaf states.
	Initial *State[T]
}

// Hooks are optional instrumentation callbacks for testing and debugging,
// corresponding to Zephyr's struct smf_hooks. Any field may be nil.
type Hooks[T any] struct {
	// OnTransition is called after the current state changes, before the
	// entry actions of the new state run.
	OnTransition func(obj T, source, dest *State[T])
	// OnAction is called before each entry, run or exit action runs.
	OnAction func(obj T, state *State[T], action Action)
	// OnError is called when an invalid operation is detected, with
	// ErrNilTransition or ErrTransitionInExit.
	OnError func(obj T, err error)
}

// Ctx is the context of a state machine operating on objects of type T,
// corresponding to Zephyr's struct smf_ctx. It is meant to be embedded in
// the user defined object T, typically as Ctx[*Object] in Object, so the
// object's actions can call SetState and SetTerminate on it directly.
//
// The zero value is ready for SetInitial.
type Ctx[T any] struct {
	obj       T
	current   *State[T]
	previous  *State[T]
	executing *State[T]
	hooks     *Hooks[T]

	terminateVal int

	// Zephyr keeps these in the internal bitfield of smf_ctx.
	newState  bool
	terminate bool
	isExit    bool
	handled   bool
}

// Current returns the current leaf state. It may be a parent state if the
// state machine is malformed, i.e. a parent state without an Initial child
// was entered directly. It corresponds to smf_get_current_leaf_state.
func (c *Ctx[T]) Current() *State[T] { return c.current }

// Previous returns the state the last transition left, or nil before the
// first transition.
func (c *Ctx[T]) Previous() *State[T] { return c.previous }

// Executing returns the state whose action is executing, which may be a
// parent of the current state. It corresponds to
// smf_get_current_executing_state.
func (c *Ctx[T]) Executing() *State[T] { return c.executing }

// SetHooks installs instrumentation hooks, or removes them if hooks is nil.
// It must be called after SetInitial, which removes any hooks; the entry
// actions SetInitial runs are therefore not reported. It corresponds to
// smf_set_hooks.
func (c *Ctx[T]) SetHooks(hooks *Hooks[T]) { c.hooks = hooks }

// SetTerminate terminates the state machine. It may be called from an entry,
// run or exit action. RunState returns val, which should be non-zero, from
// then on. It corresponds to smf_set_terminate.
func (c *Ctx[T]) SetTerminate(val int) {
	c.terminate = true
	c.terminateVal = val
}

// SetInitial resets the state machine and enters state, following Initial
// children down to a leaf. The entry actions of state and all its ancestors
// run, outermost first. obj is the object passed to every action; it is
// usually the object that embeds c. It corresponds to smf_set_initial.
func (c *Ctx[T]) SetInitial(obj T, state *State[T]) {
	// The final target is the deepest leaf state that state contains.
	for state.Initial != nil {
		state = state.Initial
	}

	c.clearInternalState()
	c.obj = obj
	c.current = state
	c.previous = nil
	c.terminateVal = 0
	c.hooks = nil

	c.executing = state
	// topmost is the root ancestor of state.
	topmost := childOf(state, nil)

	// executeAllEntryActions does not run topmost's entry action.
	if topmost.Entry != nil {
		c.executing = topmost
		c.actionHook(topmost, ActionEntry)
		topmost.Entry(c.obj)
		c.executing = state
		if c.terminate {
			return
		}
	}

	c.executeAllEntryActions(state, topmost)
}

// SetState transitions to state. The exit actions run from the current
// state up to, but not including, the least common ancestor of the executing
// state and state; then the entry actions run down to state, following
// Initial children to a leaf. A transition to the executing state itself
// runs its exit and entry actions.
//
// SetState may be called from entry and run actions. Called from an exit
// action, or with a nil state, it reports an error to [Hooks.OnError] and
// does nothing. It corresponds to smf_set_state.
func (c *Ctx[T]) SetState(state *State[T]) {
	if state == nil {
		c.errorHook(ErrNilTransition)
		return
	}

	// A transition is already under way during an exit action, so the
	// intended state would always be ignored.
	if c.isExit {
		c.errorHook(ErrTransitionInExit)
		return
	}

	var topmost *State[T]
	switch {
	case c.executing != state && c.executing.Parent == state.Parent:
		// Sibling transition: different states under the same parent.
		topmost = c.executing.Parent
	case isDescendantOf(c.executing, state):
		// state is an ancestor of where we are now.
		topmost = state
	case isDescendantOf(state, c.executing):
		// We are an ancestor of state.
		topmost = c.executing
	default:
		// Not directly related: find the least common ancestor.
		topmost = lcaOf(c.executing, state)
	}

	c.isExit = true
	c.newState = true

	// Run all exit actions up to, but not including, topmost.
	if c.executeAllExitActions(topmost) {
		return
	}

	// A transition to self runs the exit action...
	if c.executing == state && state.Exit != nil {
		c.actionHook(state, ActionExit)
		state.Exit(c.obj)
		if c.terminate {
			return
		}
	}

	c.isExit = false

	// ...and then the entry action.
	if c.executing == state && state.Entry != nil {
		c.actionHook(state, ActionEntry)
		state.Entry(c.obj)
		if c.terminate {
			return
		}
	}

	// The final target is the deepest leaf state that state contains.
	for state.Initial != nil {
		state = state.Initial
	}

	c.previous = c.current
	c.current = state
	c.executing = state

	c.transitionHook(c.previous, c.current)

	// Run all entry actions, except that of topmost.
	c.executeAllEntryActions(state, topmost)
}

// RunState runs one iteration of the state machine: the current state's Run
// action and then, while the event propagates, those of its ancestors. It
// returns zero, or the non-zero value passed to SetTerminate once the state
// machine has terminated, after which it runs no more actions. It
// corresponds to smf_run_state.
func (c *Ctx[T]) RunState() int {
	if c.terminate {
		return c.terminateVal
	}

	// A Run action may cause a transition, so clear the internal state to
	// make sure that transition is handled correctly.
	c.clearInternalState()

	c.executing = c.current
	if c.current.Run != nil {
		c.actionHook(c.current, ActionRun)
		if c.current.Run(c.obj) == Handled {
			c.handled = true
		}
	}

	if c.executeAncestorRunActions() {
		return c.terminateVal
	}
	return 0
}

// clearInternalState resets the flags Zephyr keeps in smf_ctx.internal.
func (c *Ctx[T]) clearInternalState() {
	c.isExit = false
	c.terminate = false
	c.handled = false
	c.newState = false
}

// executeAllEntryActions runs the entry actions from the direct child of
// topmost down to state. topmost's own entry action does not run. It reports
// whether the state machine terminated.
func (c *Ctx[T]) executeAllEntryActions(state, topmost *State[T]) bool {
	if state == topmost {
		// There are no child states.
		return false
	}

	for s := childOf(state, topmost); s != nil && s != state; s = childOf(state, s) {
		// Track the executing entry action in case it calls SetState.
		c.executing = s
		if s.Entry != nil {
			c.actionHook(s, ActionEntry)
			s.Entry(c.obj)
			if c.terminate {
				c.executing = c.current
				return true
			}
		}
	}

	c.executing = state
	if state.Entry != nil {
		c.actionHook(state, ActionEntry)
		state.Entry(c.obj)
		if c.terminate {
			c.executing = c.current
			return true
		}
	}

	c.executing = c.current
	return false
}

// executeAncestorRunActions runs the Run actions of the current state's
// ancestors, innermost first, until one handles the event or changes state.
// It reports whether the state machine terminated.
func (c *Ctx[T]) executeAncestorRunActions() bool {
	if c.terminate {
		return true
	}

	// The child state either transitioned or handled the event. Either
	// way, stop propagating.
	if c.newState || c.handled {
		return false
	}

	for s := c.current.Parent; s != nil; s = s.Parent {
		// Track where we are in case an ancestor calls SetState.
		c.executing = s
		if s.Run != nil {
			c.actionHook(s, ActionRun)
			if s.Run(c.obj) == Handled {
				c.handled = true
			}
			if c.terminate {
				c.executing = c.current
				return true
			}
			// This state dealt with it. Stop propagating.
			if c.newState || c.handled {
				break
			}
		}
	}

	c.executing = c.current
	return false
}

// executeAllExitActions runs the exit actions from the current state up to
// the direct child of topmost. topmost's own exit action does not run. It
// reports whether the state machine terminated.
func (c *Ctx[T]) executeAllExitActions(topmost *State[T]) bool {
	saved := c.executing

	for s := c.current; s != nil && s != topmost; s = s.Parent {
		if s.Exit != nil {
			c.executing = s
			c.actionHook(s, ActionExit)
			s.Exit(c.obj)
			if c.terminate {
				c.executing = saved
				return true
			}
		}
	}

	c.executing = saved
	return false
}

func (c *Ctx[T]) actionHook(state *State[T], action Action) {
	if c.hooks != nil && c.hooks.OnAction != nil {
		c.hooks.OnAction(c.obj, state, action)
	}
}

func (c *Ctx[T]) transitionHook(source, dest *State[T]) {
	if c.hooks != nil && c.hooks.OnTransition != nil {
		c.hooks.OnTransition(c.obj, source, dest)
	}
}

func (c *Ctx[T]) errorHook(err error) {
	if c.hooks != nil && c.hooks.OnError != nil {
		c.hooks.OnError(c.obj, err)
	}
}

// isDescendantOf reports whether test is target or one of its descendants.
func isDescendantOf[T any](test, target *State[T]) bool {
	for s := test; s != nil; s = s.Parent {
		if s == target {
			return true
		}
	}
	return false
}

// childOf returns the ancestor of state (or state itself) whose parent is
// parent, or nil if there is none. With a nil parent it returns the root
// ancestor of state.
func childOf[T any](state, parent *State[T]) *State[T] {
	for s := state; s != nil; s = s.Parent {
		if s.Parent == parent {
			return s
		}
	}
	return nil
}

// lcaOf returns the least common ancestor of source and dest, which are not
// ancestors of one another, or nil if they have none.
func lcaOf[T any](source, dest *State[T]) *State[T] {
	for a := source.Parent; a != nil; a = a.Parent {
		if isDescendantOf(dest, a) {
			return a
		}
	}
	return nil
}
