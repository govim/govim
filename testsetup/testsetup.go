// Package testsetup defines some test-based constants that are common to
// tests and CI setup
package testsetup

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/govim/govim"
)

// dev environment variables
const (
	EnvTestSocket            = "GOVIMTEST_SOCKET"
	EnvVimFlavor             = "VIM_FLAVOR"
	EnvVimCommand            = "VIM_COMMAND"
	EnvGithubUser            = "GH_USER"
	EnvGithubToken           = "GH_TOKEN"
	EnvLoadTestAPI           = "GOVIM_LOAD_TEST_API"
	EnvTestscriptStderr      = "GOVIM_TESTSCRIPT_STDERR"
	EnvTestscriptWorkdirRoot = "GOVIM_TESTSCRIPT_WORKDIR_ROOT"
	EnvErrLogMatchWait       = "GOVIM_ERRLOGMATCH_WAIT"

	// EnvDisableUserBusy is used in a test environment to disable the the
	// normal timeout-based shifting from user busy <-> user not busy.  This is
	// largely safe on the basis that we block/wait in tests as required.
	// Setting this variable to "true" (which should only be done in tests) also
	// causes a function to be declared in Vim that allows the manual switching
	// from user busy <-> user not busy.
	EnvDisableUserBusy = "GOVIM_DISABLE_USER_BUSY"

	// EnvTestscriptIssues can be set to a regular expression which
	// causes issue tracker conditions not to be satisfied. e.g.
	// GOVIM_TESTSCRIPT_ISSUES=. will cause all issue tracker conditions
	// (e.g. [golang.org/issues/1234]) to not be satisfied.
	EnvTestscriptIssues = "GOVIM_TESTSCRIPT_ISSUES"

	// EnvTestRaceSlowdown is a floating point factor by which to adjust
	// EnvErrLogMatchWait for race tests
	EnvTestRaceSlowdown = "GOVIM_TEST_RACE_SLOWDOWN"
)

// user environment variables
const (
	EnvLogfileTmpl = "GOVIM_LOGFILE_TMPL"
)

// vim versions
const (
	// MinVimGovim represents the bare minimum version of Vim required to
	// use govim
	MinVimGovim = "v8.1.1711"

	// MinVimSafeState is the minimum version required to use Vim's state()
	// and SafeState* functionality.
	MinVimSafeState = "v8.1.2056"

	LatestVim = "v8.2.2385"
)

var (
	VimCommand  = Command{"vim"}
	GvimCommand = Command{"xvfb-run", "-a", "gvim", "-f"}
)

type Command []string

func (c Command) String() string {
	return strings.Join(c, " ")
}

func (c Command) BuildCommand(args ...string) *exec.Cmd {
	res := exec.Command(c[0], c[1:]...)
	res.Args = append(res.Args, args...)
	return res
}

var (
	GoVersions = []string{"go1.15.8", "go1.16"}

	// VimVersions contains the versions of all flavors of Vim/Gvim/X to be tested
	VimVersions = []Version{
		VimVersion(MinVimGovim),
		GvimVersion(MinVimGovim),
		VimVersion(MinVimSafeState),
		GvimVersion(MinVimSafeState),
		VimVersion(LatestVim),
		GvimVersion(LatestVim),
	}
)

type Version interface {
	Version() string
	Command() string
	Flavor() govim.Flavor
}

type baseVersionType struct {
	v string
}

func (b baseVersionType) Version() string {
	return b.v
}

func VimVersion(v string) vimVersionType {
	return vimVersionType{baseVersionType: baseVersionType{v: v}}
}

type vimVersionType struct {
	baseVersionType
}

func (v vimVersionType) Command() string {
	return strings.Join(VimCommand, " ")
}

func (v vimVersionType) Flavor() govim.Flavor {
	return govim.FlavorVim
}

func GvimVersion(v string) gvimVersionType {
	return gvimVersionType{baseVersionType: baseVersionType{v: v}}
}

type gvimVersionType struct {
	baseVersionType
}

func (v gvimVersionType) Command() string {
	return strings.Join(GvimCommand, " ")
}

func (v gvimVersionType) Flavor() govim.Flavor {
	return govim.FlavorGvim
}

func EnvLookupFlavorCommand() (flav govim.Flavor, cmd Command, err error) {
	vf, ok := os.LookupEnv("VIM_FLAVOR")
	if !ok {
		return flav, cmd, fmt.Errorf("VIM_FLAVOR env var is not set")
	}
	foundFlav := false
	for _, f := range govim.Flavors {
		if f.String() == vf {
			flav = f
			foundFlav = true
		}
	}
	if !foundFlav {
		return flav, cmd, fmt.Errorf("VIM_FLAVOR set to invalid value: %v", vf)
	}
	switch flav {
	case govim.FlavorVim:
		cmd = VimCommand
	case govim.FlavorGvim:
		cmd = GvimCommand
	}
	return flav, cmd, nil
}
