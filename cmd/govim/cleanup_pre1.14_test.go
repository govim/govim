// +build !go1.14

package main

import "testing"

func cleanup(t *testing.T, f func()) {
	// This is a no-op pre 1.14
}
