// Package testsetup defines some test-based constants that are common to
// tests and CI setup
package testsetup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	// EnvTestPathEnv is used to override PATH when looking for executables.
	// Testscript prepends PATH with the subcommand folder, so EnvTestPathEnv
	// acts as a snapshot of PATH before it was modified.
	EnvTestPathEnv = "GOVIM_TEST_PATH_ENV"
)

// vim versions
const (
	LatestVim = "v9.0.2112"
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
	GoVersions = []string{"go1.20.11", "go1.21.4"}

	// VimVersions contains the versions of all flavors of Vim/Gvim/X to be tested
	VimVersions = []Version{
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
	vf, ok := os.LookupEnv(EnvVimFlavor)
	if !ok {
		return flav, cmd, fmt.Errorf("%v env var is not set", EnvVimFlavor)
	}
	foundFlav := false
	for _, f := range govim.Flavors {
		if f.String() == vf {
			flav = f
			foundFlav = true
		}
	}
	if !foundFlav {
		return flav, cmd, fmt.Errorf("%v set to invalid value: %v", EnvVimFlavor, vf)
	}
	switch flav {
	case govim.FlavorVim:
		cmd = VimCommand
	case govim.FlavorGvim:
		cmd = GvimCommand
	}
	return flav, cmd, nil
}

// LookPath searches for an executable named file, using EnvTestPathEnv as PATH
// (if set). The ordinary PATH env is used if it isn't set.
// If file contains a slash, it is tried directly.
// The result may be an absolute path or a path relative to the current directory.
func LookPath(file string) (string, error) {
	findExecutable := func(file string) error {
		d, err := os.Stat(file)
		if err != nil {
			return err
		}
		if m := d.Mode(); !m.IsDir() && m&0111 != 0 {
			return nil
		}
		return os.ErrPermission
	}

	if strings.Contains(file, "/") {
		err := findExecutable(file)
		if err == nil {
			return file, nil
		}
		return "", err
	}

	pathEnv := os.Getenv(EnvTestPathEnv)
	if pathEnv == "" {
		pathEnv = os.Getenv("PATH")
	}
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		path := filepath.Join(dir, file)
		if err := findExecutable(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("failed to find executable in provided path")
}
