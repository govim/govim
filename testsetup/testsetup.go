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

const (
	EnvTestSocket = "GOVIMTEST_SOCKET"

	EnvVimFlavor  = "VIM_FLAVOR"
	EnvVimCommand = "VIM_COMMAND"

	EnvGithubUser  = "GH_USER"
	EnvGithubToken = "GH_TOKEN"

	EnvLoadTestAPI      = "GOVIM_LOAD_TEST_API"
	EnvDisableSignPlace = "GOVIM_DISABLE_SIGNPLACE"

	// MinVimGovim represents the bare minimum version of Vim required to
	// use govim
	MinVimGovim = "v8.1.1711"

	EnvLogfileTmpl      = "GOVIM_LOGFILE_TMPL"
	EnvTestscriptStderr = "GOVIM_TESTSCRIPT_STDERR"

	LatestVim = "v8.1.1991"
)

var (
	VimCommand    = Command{"vim"}
	GvimCommand   = Command{"xvfb-run", "-a", "gvim", "-f"}
	NeovimCommand = Command{"nvim"}
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
	GoVersions = []string{"go1.12.9", "go1.13"}

	// VimVersions contains the versions of all flavors of Vim/Gvim/X to be tested
	VimVersions = []Version{
		NeovimVersion("v0.3.5"),
		VimVersion(MinVimGovim),
		GvimVersion(MinVimGovim),
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

func NeovimVersion(v string) neovimVersionType {
	return neovimVersionType{baseVersionType: baseVersionType{v: v}}
}

type neovimVersionType struct {
	baseVersionType
}

func (v neovimVersionType) Command() string {
	return strings.Join(NeovimCommand, " ")
}

func (v neovimVersionType) Flavor() govim.Flavor {
	return govim.FlavorNeovim
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
