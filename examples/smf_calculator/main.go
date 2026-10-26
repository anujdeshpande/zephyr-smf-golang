// Copyright (c) 2024 Glenn Andrews
// Copyright 2026 Anuj Deshpande
//
// SPDX-License-Identifier: Apache-2.0

// Command smf_calculator is a basic desk calculator driven by a state
// machine, ported from Zephyr's samples/subsys/smf/smf_calculator.
//
// The business logic follows the statechart in Fig 2.18 of Practical UML
// Statecharts in C/C++, 2nd Edition, by Miro Samek. It is a three-layer
// hierarchical statechart that handles situations such as ignoring leading
// zeroes and disallowing multiple decimal points. As in the Zephyr sample,
// the op_entered state shows different output depending on whether a
// previous result is available.
//
// The Zephyr sample takes keys from an LVGL touchscreen and from the shell.
// This port takes them from standard input: every character typed is a key,
// and the display is printed after each one. Valid keys are 0-9, '.', + - * /,
// '=', C (cancel) and E (cancel entry).
//
//	$ echo '12+30=' | smf_calculator
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

func main() {
	calc := newCalculator()
	in := bufio.NewReader(os.Stdin)
	for {
		c, err := in.ReadByte()
		if err == io.EOF {
			return
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			continue
		}
		ev, ok := keyEvent(c)
		if !ok {
			fmt.Fprintf(os.Stderr, "invalid key %q\n", c)
			continue
		}
		out, running := calc.press(ev)
		if !running {
			return
		}
		fmt.Printf("%c  [%16s]\n", c, out)
	}
}
