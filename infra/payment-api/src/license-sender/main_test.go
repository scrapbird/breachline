package main

import "testing"

func TestDeliverableAddress(t *testing.T) {
	cases := map[string]string{
		"stripe@example.com":     sesSimulatorSuccess,
		"a@sub.EXAMPLE.org":      sesSimulatorSuccess,
		"user@foo.test":          sesSimulatorSuccess,
		"user@host.invalid":      sesSimulatorSuccess,
		"customer@gmail.com":     "customer@gmail.com",
		"someone@notexample.com": "someone@notexample.com",
		"info@breachline.app":    "info@breachline.app",
		"no-at-sign":             "no-at-sign",
	}
	for in, want := range cases {
		if got := deliverableAddress(in); got != want {
			t.Errorf("deliverableAddress(%q) = %q, want %q", in, got, want)
		}
	}
}
