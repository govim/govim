package testdriver

import (
	"errors"
	"os"
	"testing"

	"github.com/govim/govim/testsetup"
	"github.com/rogpeppe/go-internal/semver"
)

func TestCondition(t *testing.T) {
	t.Cleanup(cleanupEnvVars(testsetup.EnvTestscriptIssues))
	t.Cleanup(cleanupEnvVars(testsetup.EnvVimFlavor))

	_, cmd, err := testsetup.EnvLookupFlavorCommand()
	if err != nil {
		t.Fatalf("failed to derive flavor command: %v", err)
	}
	vimVersion, err := getVimFlavourVersion(cmd)
	if err != nil {
		t.Fatalf("failed to get vim flavour version: %v", err)
	}

	var conditionTests = []struct {
		desc      string
		flavorenv *string
		issueenv  *string
		cond      string
		satisfied bool
		err       error
	}{
		{
			desc:     "match all golang issues",
			issueenv: pS("."),
			cond:     "golang.org/issues/1234",
		},
		{
			desc:     "match all govim issues",
			issueenv: pS("."),
			cond:     "github.com/govim/govim/issues/1234",
		},
		{
			desc:      "bad issue condition",
			issueenv:  pS("."),
			flavorenv: pS("vim"),
			cond:      "github.com/apple/pear/issues/1234",
			satisfied: false,
			err:       errors.New("unknown condition github.com/apple/pear/issues/1234"),
		},
		{
			desc:      "vim test",
			flavorenv: pS("vim"),
			cond:      "vim",
			satisfied: true,
		},
		{
			desc:      "gvim test",
			flavorenv: pS("gvim"),
			cond:      "gvim",
			satisfied: true,
		},
		{
			desc:      "gvim test running with vim",
			flavorenv: pS("vim"),
			cond:      "gvim",
		},
		{
			desc:      "vim test running with gvim",
			flavorenv: pS("gvim"),
			cond:      "vim",
		},
		{
			// This is a bit like marking our own homework because we
			// are contriving the result of satisfied, but that's fine;
			// all we are looking to do is exercise the condition
			// matching logic.
			desc:      "vim version exercise",
			flavorenv: pS("vim"),
			cond:      "v8.2.3333",
			satisfied: semver.Compare(vimVersion, "v8.2.3333") >= 0,
		},
	}

	for _, tc := range conditionTests {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.issueenv != nil {
				os.Setenv(testsetup.EnvTestscriptIssues, *tc.issueenv)
			}
			if tc.flavorenv != nil {
				os.Setenv(testsetup.EnvVimFlavor, *tc.flavorenv)
			}
			got, err := Condition(tc.cond)
			if got != tc.satisfied {
				t.Errorf("wanted %v; got %v", tc.satisfied, got)
			}
			if err == nil {
				if tc.err != nil {
					t.Errorf("expected error %q; got none", tc.err)
				}
			} else {
				if tc.err == nil {
					t.Errorf("did not expect error; got %q", err)
				} else {
					if err.Error() != tc.err.Error() {
						t.Errorf("got error %q; wanted %q", err, tc.err)
					}
				}
			}
			os.Unsetenv(testsetup.EnvTestscriptIssues)
			os.Unsetenv(testsetup.EnvVimFlavor)
		})
	}
}

func pS(s string) *string {
	return &s
}

func cleanupEnvVars(ev string) func() {
	v, ok := os.LookupEnv(ev)
	return func() {
		if ok {
			os.Setenv(ev, v)
		} else {
			os.Unsetenv(ev)
		}
	}
}
