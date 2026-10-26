// Copyright 2026 Anuj Deshpande
// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestCalculator(t *testing.T) {
	for _, tc := range []struct {
		keys, want string
	}{
		{"", " 0"},
		{"1+2=", " 3"},
		{"12+30=", " 42"},
		{"2*3=", " 6"},
		{"7/2=", " 3.5"},
		{"1/0=", "ERROR"},
		{"1/0=C", " 0"},
		{"007", " 7"},      // leading zeroes are ignored
		{"1.2.3", " 1.23"}, // so are further decimal points
		{".5", " .5"},
		{"-5", "-5"},  // negated first operand
		{"--5", "-5"}, // a second '-' is ignored
		{"-5+2=", "-3"},
		{"5*-2=", "-10"}, // negated second operand
		{"1+2+", " 3"},   // chaining shows the running result
		{"1+2+4=", " 7"},
		{"1+2=*4=", " 12"}, // an operator after '=' chains too
		{"12E", " 0"},      // cancel entry clears operand 1
		{"12+34E", " 0"},   // and operand 2
		{"12+34E5=", " 17"},
		{"1234567890123456789", " 123456789012345"}, // at most 15 digits
	} {
		calc := newCalculator()
		got := calc.obj.display()
		for i := range len(tc.keys) {
			ev, ok := keyEvent(tc.keys[i])
			if !ok {
				t.Fatalf("%q: bad key %q", tc.keys, tc.keys[i])
			}
			got, ok = calc.press(ev)
			if !ok {
				t.Fatalf("%q: state machine terminated", tc.keys)
			}
		}
		if got != tc.want {
			t.Errorf("%q: display %q, want %q", tc.keys, got, tc.want)
		}
	}
}
