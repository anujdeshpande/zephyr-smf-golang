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

// Command hsm_psicc2 is a hierarchical state machine demo based on the
// example in PSiCC2, ported from Zephyr's samples/subsys/smf/hsm_psicc2.
//
// It implements the statechart in Figure 2.11 of Practical UML Statecharts
// in C/C++, 2nd Edition, by Miro Samek. The chart contains all possible
// transition topologies up to four levels of state nesting. Each entry, run
// and exit action is logged, as is each event a state handles or passes up
// to its parent.
//
// Commands are read from standard input, one per line, in place of the
// Zephyr shell:
//
//	event <A-I>   send event A to I of PSiCC2 Figure 2.11
//	terminate     terminate the state machine; later events are ignored
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Println("State Machine Framework Demo")
	fmt.Println("See PSiCC2 Fig 2.11 for the statechart")
	fmt.Println("https://www.state-machine.com/psicc2")
	fmt.Println()

	events := make(chan event, 10)
	done := make(chan struct{})
	go func() {
		run(events, os.Stdout)
		close(done)
	}()

	in := bufio.NewScanner(os.Stdin)
	for in.Scan() {
		ev, err := parse(in.Text())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		if ev < 0 {
			continue
		}
		select {
		case events <- ev:
		case <-done:
			// Future events are ignored once the machine terminates.
		}
	}
	close(events)
	<-done
}

// parse reads one command. It returns -1 for a blank line.
func parse(line string) (event, error) {
	f := strings.Fields(line)
	switch {
	case len(f) == 0:
		return -1, nil
	case len(f) == 1 && f[0] == "terminate":
		return eventTerminate, nil
	case len(f) == 2 && f[0] == "event" && len(f[1]) == 1:
		c := strings.ToUpper(f[1])[0]
		if c >= 'A' && c <= 'I' {
			return eventA + event(c-'A'), nil
		}
		return -1, fmt.Errorf("invalid argument %s", f[1])
	}
	return -1, fmt.Errorf("usage: event <A-I> | terminate")
}
