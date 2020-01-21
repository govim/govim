// +build go1.14

package main

import "testing"

func cleanup(t *testing.T, f func()) {
	// TODO when there is a 1.14 release which includes CL 214822 we can
	// uncomment the next line
	// t.Cleanup(f)
}
