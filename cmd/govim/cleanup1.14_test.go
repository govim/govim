// +build go1.14

package main

import "testing"

func cleanup(t *testing.T, f func()) {
	t.Cleanup(f)
}
