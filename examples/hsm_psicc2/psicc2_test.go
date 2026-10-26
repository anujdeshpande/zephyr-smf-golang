// Copyright 2026 Anuj Deshpande
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// The expected traces follow PSiCC2 Figure 2.11 and the SMF transition
// rules: exits up to the least common ancestor, entries down to the target,
// then initial transitions down to a leaf.
func TestPSiCC2(t *testing.T) {
	events := make(chan event, 10)
	for _, e := range []event{eventG, eventI, eventA, eventTerminate, eventB} {
		events <- e
	}
	close(events)

	var log strings.Builder
	run(events, &log)

	want := []string{
		// Initial transition: initial -> s2 -> s211.
		"initial_entry", "s_entry", "s2_entry", "s21_entry", "s211_entry",
		// G: s21 handles it; LCA is s; s1's initial transition goes to s11.
		"s211_run", "s21_run", "s21_run received EVENT_G",
		"s211_exit", "s21_exit", "s2_exit", "s1_entry", "s11_entry",
		// I: s1 handles it, so s does not run.
		"s11_run", "s1_run", "s1_run received EVENT_I",
		// A: transition to self in s1 exits and re-enters s1.
		"s11_run", "s1_run", "s1_run received EVENT_A",
		"s11_exit", "s1_exit", "s1_entry", "s11_entry",
		// Terminate: s handles it. Event B is never run.
		"s11_run", "s1_run", "s_run", "s_run received SMF_EVENT_TERMINATE. Terminating",
		"hsm_psicc2_thread terminating state machine thread",
	}
	got := strings.Split(strings.TrimSuffix(log.String(), "\n"), "\n")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("trace:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestParse(t *testing.T) {
	for in, want := range map[string]event{
		"event a":   eventA,
		"event I":   eventI,
		"terminate": eventTerminate,
		"   ":       -1,
	} {
		got, err := parse(in)
		if err != nil || got != want {
			t.Errorf("parse(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"event J", "event", "foo"} {
		if _, err := parse(in); err == nil {
			t.Errorf("parse(%q) succeeded, want an error", in)
		}
	}
}
