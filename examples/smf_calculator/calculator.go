// Copyright (c) 2024 Glenn Andrews
// Copyright 2026 Anuj Deshpande
//
// SPDX-License-Identifier: Apache-2.0

// Ported from samples/subsys/smf/smf_calculator/src/smf_calculator_thread.c.

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/anujdeshpande/zephyr-smf-golang/smf"
)

const (
	calculatorMaxDigits = 15
	// One byte for the '-' sign and, in C, one for the trailing NUL.
	calculatorStringLength = calculatorMaxDigits + 2
)

type eventID int

const (
	digit0 eventID = iota
	digit1to9
	decimalPoint
	operator
	equals
	cancelEntry
	cancelButton
)

type calculatorEvent struct {
	id      eventID
	operand byte
}

// keyEvent maps a key to its event, as the sample's shell command does:
// 0-9, '.', + - * /, '=', C (cancel) and E (cancel entry).
func keyEvent(c byte) (calculatorEvent, bool) {
	ev := calculatorEvent{operand: c}
	switch {
	case c >= '1' && c <= '9':
		ev.id = digit1to9
	case c == '0':
		ev.id = digit0
	case c == '.':
		ev.id = decimalPoint
	case c == '+' || c == '-' || c == '*' || c == '/':
		ev.id = operator
	case c == '=':
		ev.id = equals
	case c == 'C' || c == 'c':
		ev.id = cancelButton
	case c == 'E' || c == 'e':
		ev.id = cancelEntry
	default:
		return ev, false
	}
	return ev, true
}

type displayMode int

const (
	displayOperand1 displayMode = iota
	displayOperand2
	displayResult
	displayError
)

// operand is a number as typed. Position 0 is the sign; a '0' at index 1
// shows until the first digit overwrites it.
type operand struct {
	chars []byte
	index int
}

func (op *operand) setup() {
	// Indexes start at 1 because position 0 is the sign.
	op.index = 1
	op.chars = []byte{' ', '0'}
}

func (op *operand) insert(digit byte) {
	if op.index >= calculatorStringLength-1 {
		// Can't store an extra digit.
		return
	}
	op.chars = append(op.chars[:op.index], digit)
	op.index++
}

func (op *operand) negate() {
	if op.chars[0] == ' ' {
		op.chars[0] = '-'
	} else {
		op.chars[0] = ' '
	}
}

func (op *operand) String() string { return string(op.chars) }

// copyFrom copies src, which is assumed to be well formed.
func (op *operand) copyFrom(src *operand) {
	op.chars = append([]byte(nil), src.chars...)
	op.index = src.index
}

// value parses the operand the way strtod does: the longest valid prefix,
// or 0 if there is none.
func (op *operand) value() float64 {
	s := strings.TrimSpace(op.String())
	for n := len(s); n > 0; n-- {
		if v, err := strconv.ParseFloat(s[:n], 64); err == nil {
			return v
		}
	}
	return 0
}

// object is the user defined object.
type object struct {
	smf.Ctx[*object]

	// Other state specific data.
	event       calculatorEvent
	operand1    operand
	operand2    operand
	operatorBtn byte
	result      operand
	displayMode displayMode
}

// calculateResult applies the operator. It reports false for a division by
// zero or an unknown operator.
func (s *object) calculateResult() bool {
	op1, op2 := s.operand1.value(), s.operand2.value()
	var result float64
	switch s.operatorBtn {
	case '+':
		result = op1 + op2
	case '-':
		result = op1 - op2
	case '*':
		result = op1 * op2
	case '/':
		if op2 == 0 {
			// Division by zero.
			return false
		}
		result = op1 / op2
	default:
		// Unknown operator.
		return false
	}

	str := fmt.Sprintf("% f", result)
	// Strip trailing zeroes from the result, and the point if it was .000.
	str = strings.TrimRight(str, "0")
	str = strings.TrimSuffix(str, ".")
	// The C sample copies at most CALCULATOR_STRING_LENGTH - 1 bytes.
	if len(str) > calculatorStringLength-1 {
		str = str[:calculatorStringLength-1]
	}
	s.result.chars = []byte(str)
	s.result.index = len(str)
	return true
}

// chainCalculations copies the result into operand 1 and clears operand 2
// to allow chaining calculations.
func (s *object) chainCalculations() {
	s.operand1.copyFrom(&s.result)
	s.operand2.setup()
}

// Possible states.
const (
	stateOn = iota
	stateReady
	stateResult
	stateBegin
	stateNegated1
	stateOperand1
	stateZero1
	stateInt1
	stateFrac1
	stateNegated2
	stateOperand2
	stateZero2
	stateInt2
	stateFrac2
	stateOpEntered
	stateOpChained
	stateOpNormal
	stateError
)

var calculatorStates []smf.State[*object]

func init() {
	st := make([]smf.State[*object], stateError+1)
	st[stateOn] = smf.State[*object]{Entry: onEntry, Run: onRun, Initial: &st[stateReady]}
	st[stateReady] = smf.State[*object]{Run: readyRun, Parent: &st[stateOn], Initial: &st[stateBegin]}
	st[stateResult] = smf.State[*object]{Entry: resultEntry, Run: resultRun, Parent: &st[stateReady]}
	st[stateBegin] = smf.State[*object]{Entry: beginEntry, Run: beginRun, Parent: &st[stateReady]}
	st[stateNegated1] = smf.State[*object]{Entry: negated1Entry, Run: negated1Run, Parent: &st[stateOn]}
	st[stateOperand1] = smf.State[*object]{Entry: operand1Entry, Run: operand1Run, Parent: &st[stateOn]}
	st[stateZero1] = smf.State[*object]{Run: zero1Run, Parent: &st[stateOperand1]}
	st[stateInt1] = smf.State[*object]{Run: int1Run, Parent: &st[stateOperand1]}
	st[stateFrac1] = smf.State[*object]{Run: frac1Run, Parent: &st[stateOperand1]}
	st[stateNegated2] = smf.State[*object]{Entry: negated2Entry, Run: negated2Run, Parent: &st[stateOn]}
	st[stateOperand2] = smf.State[*object]{Entry: operand2Entry, Run: operand2Run, Parent: &st[stateOn]}
	st[stateZero2] = smf.State[*object]{Run: zero2Run, Parent: &st[stateOperand2]}
	st[stateInt2] = smf.State[*object]{Run: int2Run, Parent: &st[stateOperand2]}
	st[stateFrac2] = smf.State[*object]{Run: frac2Run, Parent: &st[stateOperand2]}
	st[stateOpEntered] = smf.State[*object]{Run: opEnteredRun, Parent: &st[stateOn], Initial: &st[stateOpNormal]}
	st[stateOpChained] = smf.State[*object]{Entry: opChainedEntry, Parent: &st[stateOpEntered]}
	st[stateOpNormal] = smf.State[*object]{Entry: opNormalEntry, Parent: &st[stateOpEntered]}
	st[stateError] = smf.State[*object]{Entry: errorEntry, Parent: &st[stateOn]}
	calculatorStates = st
}

func (s *object) setState(state int) { s.SetState(&calculatorStates[state]) }

func onEntry(s *object) {
	s.operand1.setup()
	s.operand2.setup()
	s.result.setup()
	s.operatorBtn = 0
}

func onRun(s *object) smf.Result {
	if s.event.id == cancelButton {
		s.setState(stateOn)
	}
	return smf.Propagate
}

func readyRun(s *object) smf.Result {
	switch s.event.id {
	case decimalPoint:
		s.operand1.insert(s.event.operand)
		s.setState(stateFrac1)
	case digit1to9:
		s.operand1.insert(s.event.operand)
		s.setState(stateInt1)
	case digit0:
		// Don't insert the leading zero.
		s.setState(stateZero1)
	case operator:
		s.operatorBtn = s.event.operand
		s.setState(stateOpChained)
	}
	return smf.Propagate
}

func resultEntry(s *object) { s.displayMode = displayResult }

func resultRun(*object) smf.Result {
	// The ready state handles the exits from this state.
	return smf.Propagate
}

func beginEntry(s *object) { s.displayMode = displayOperand1 }

func beginRun(s *object) smf.Result {
	// We only care about negative.
	if s.event.id == operator && s.event.operand == '-' {
		s.setState(stateNegated1)
	}
	return smf.Propagate
}

func negated1Entry(s *object) { s.operand1.negate() }

func negated1Run(s *object) smf.Result {
	switch s.event.id {
	case decimalPoint:
		s.operand1.insert(s.event.operand)
		s.setState(stateFrac1)
	case digit1to9:
		s.operand1.insert(s.event.operand)
		s.setState(stateInt1)
	case digit0:
		// Don't need to insert leading zeroes.
		s.setState(stateZero1)
	case operator:
		// We only care about ignoring the negative key.
		if s.event.operand == '-' {
			return smf.Handled
		}
	case cancelEntry:
		s.operand1.setup()
		s.setState(stateBegin)
	}
	return smf.Propagate
}

func operand1Entry(s *object) { s.displayMode = displayOperand1 }

func operand1Run(s *object) smf.Result {
	switch s.event.id {
	case operator:
		s.operatorBtn = s.event.operand
		s.setState(stateOpEntered)
	case cancelEntry:
		s.operand1.setup()
		s.setState(stateReady)
	}
	return smf.Propagate
}

func zero1Run(s *object) smf.Result {
	switch s.event.id {
	case digit0:
		// We ignore leading zeroes.
		return smf.Handled
	case digit1to9:
		s.operand1.insert(s.event.operand)
		s.setState(stateInt1)
	case decimalPoint:
		s.operand1.insert(s.event.operand)
		s.setState(stateFrac1)
	}
	return smf.Propagate
}

func int1Run(s *object) smf.Result {
	switch s.event.id {
	case digit0, digit1to9:
		s.operand1.insert(s.event.operand)
		return smf.Handled
	case decimalPoint:
		s.operand1.insert(s.event.operand)
		s.setState(stateFrac1)
	}
	return smf.Propagate
}

func frac1Run(s *object) smf.Result {
	switch s.event.id {
	case digit0, digit1to9:
		s.operand1.insert(s.event.operand)
		return smf.Handled
	case decimalPoint:
		// Ignore further decimal points.
		return smf.Handled
	}
	return smf.Propagate
}

func negated2Entry(s *object) { s.operand2.negate() }

func negated2Run(s *object) smf.Result {
	switch s.event.id {
	case decimalPoint:
		s.operand2.insert(s.event.operand)
		s.setState(stateFrac2)
	case digit1to9:
		s.operand2.insert(s.event.operand)
		s.setState(stateInt2)
	case digit0:
		// Don't insert the leading zero.
		s.setState(stateZero2)
	case operator:
		// We only care about ignoring the negative key.
		if s.event.operand == '-' {
			return smf.Handled
		}
	case cancelEntry:
		s.operand2.setup()
		s.setState(stateOpEntered)
	}
	return smf.Propagate
}

func operand2Entry(s *object) { s.displayMode = displayOperand2 }

func operand2Run(s *object) smf.Result {
	switch s.event.id {
	case cancelEntry:
		s.operand2.setup()
		s.setState(stateOpEntered)
	case operator:
		if s.calculateResult() {
			// Move the result into operand 1 to allow chaining.
			s.chainCalculations()
			// Copy in the new operator.
			s.operatorBtn = s.event.operand
			s.setState(stateOpChained)
		} else {
			s.setState(stateError)
		}
	case equals:
		if s.calculateResult() {
			// Move the result into operand 1 to allow chaining.
			s.chainCalculations()
			s.setState(stateResult)
		} else {
			s.setState(stateError)
		}
	}
	return smf.Propagate
}

func zero2Run(s *object) smf.Result {
	switch s.event.id {
	case digit0:
		// We ignore leading zeroes.
		return smf.Handled
	case digit1to9:
		s.operand2.insert(s.event.operand)
		s.setState(stateInt2)
	case decimalPoint:
		s.operand2.insert(s.event.operand)
		s.setState(stateFrac2)
	}
	return smf.Propagate
}

func int2Run(s *object) smf.Result {
	switch s.event.id {
	case digit0, digit1to9:
		s.operand2.insert(s.event.operand)
		return smf.Handled
	case decimalPoint:
		s.operand2.insert(s.event.operand)
		s.setState(stateFrac2)
	}
	return smf.Propagate
}

func frac2Run(s *object) smf.Result {
	switch s.event.id {
	case digit0, digit1to9:
		s.operand2.insert(s.event.operand)
		return smf.Handled
	case decimalPoint:
		// Ignore further decimal points.
		return smf.Handled
	}
	return smf.Propagate
}

func opEnteredRun(s *object) smf.Result {
	switch s.event.id {
	case digit0:
		// Don't insert the leading zero.
		s.setState(stateZero2)
	case digit1to9:
		s.operand2.insert(s.event.operand)
		s.setState(stateInt2)
	case decimalPoint:
		s.operand2.insert(s.event.operand)
		s.setState(stateFrac2)
	case operator:
		// We only care about negative.
		if s.event.operand == '-' {
			s.setState(stateNegated2)
		}
	}
	return smf.Propagate
}

func opChainedEntry(s *object) { s.displayMode = displayOperand1 }

func opNormalEntry(s *object) { s.displayMode = displayOperand2 }

func errorEntry(s *object) { s.displayMode = displayError }

// display is the text the calculator shows, output_display in the sample.
func (s *object) display() string {
	switch s.displayMode {
	case displayOperand1:
		return s.operand1.String()
	case displayOperand2:
		return s.operand2.String()
	case displayResult:
		return s.result.String()
	case displayError:
		return "ERROR"
	}
	return ""
}

// calculator wraps the state machine; newCalculator is the start of
// smf_calculator_thread.
type calculator struct{ obj object }

func newCalculator() *calculator {
	c := &calculator{}
	c.obj.SetInitial(&c.obj, &calculatorStates[stateOn])
	return c
}

// press runs the state machine with one event and returns the new display.
// It reports false once the state machine has terminated.
func (c *calculator) press(ev calculatorEvent) (string, bool) {
	c.obj.event = ev
	if c.obj.RunState() != 0 {
		// The state machine was terminated.
		return "", false
	}
	return c.obj.display(), true
}
