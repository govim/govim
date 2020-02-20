package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/govim/govim"
	"github.com/govim/govim/testsetup"
	"github.com/rogpeppe/go-internal/semver"
)

var thematrix []build

type build struct {
	goversion  string
	vimversion string
	vimflavor  govim.Flavor
	vimcommand string
	env        map[string]string
}

func (b build) dup() build {
	res := b
	res.env = make(map[string]string)
	for k, v := range b.env {
		res.env[k] = v
	}
	return res
}

type matrixstep func(build) []build

func buildmatrix() []build {
	if thematrix != nil {
		return thematrix
	}
	for _, v := range testsetup.VimVersions {
		thematrix = append(thematrix, build{
			vimversion: v.Version(),
			vimflavor:  v.Flavor(),
			vimcommand: v.Command(),
		})
	}
	steps := []matrixstep{
		expGoVersions,
	}
	for _, step := range steps {
		for i := 0; i < len(thematrix); {
			var newmat []build
			newmat = append(newmat, thematrix[:i]...)
			b := thematrix[i]
			var post []build
			if i < len(thematrix)-1 {
				post = thematrix[i+1:]
			}
			nb := step(b)
			i += len(nb)
			newmat = append(newmat, nb...)
			newmat = append(newmat, post...)
			thematrix = newmat
		}
	}
	return thematrix
}

func expGoVersions(b build) (res []build) {
	for _, v := range testsetup.GoVersions {
		gv := b.dup()
		gv.goversion = v
		res = append(res, gv)
	}
	return
}

// genconfig is a very basic templater that removes the need for hand-maintaining
// a couple of files. It is the source of the build matrix in the Travis config,
// and also the source of the default versions used in buildGovimImage.sh. Should
// be run from the root of the repo
func main() {
	writeMaxVersions()
	writeGithubActionsYaml()
}

func writeMaxVersions() {
	var vs struct {
		MaxGoVersion     string
		MaxVimVersion    string
		MaxGvimVersion   string
		MaxNeovimVersion string
		GoVersions       string
		VimVersions      string
		GvimVersions     string
		NeovimVersions   string
		VimCommand       string
		GvimCommand      string
		NeovimCommand    string
		ValidFlavors     string
	}
	vs.VimCommand = strconv.Quote(testsetup.VimCommand.String())
	vs.GvimCommand = strconv.Quote(testsetup.GvimCommand.String())
	vs.NeovimCommand = strconv.Quote(testsetup.NeovimCommand.String())
	vs.MaxGoVersion = testsetup.GoVersions[len(testsetup.GoVersions)-1]

	goVersionsSet := make(map[string]bool)
	vimVersionsSet := make(map[string]bool)
	gvimVersionsSet := make(map[string]bool)
	neovimVersionsSet := make(map[string]bool)

	for _, b := range buildmatrix() {
		goVersionsSet[b.goversion] = true
		switch b.vimflavor {
		case govim.FlavorVim:
			vimVersionsSet[b.vimversion] = true
		case govim.FlavorGvim:
			gvimVersionsSet[b.vimversion] = true
		case govim.FlavorNeovim:
			neovimVersionsSet[b.vimversion] = true
		default:
			panic(fmt.Errorf("don't know about flavor %v", b.vimflavor))
		}
	}

	goVersions := setToList(goVersionsSet)
	vimVersions := setToList(vimVersionsSet)
	gvimVersions := setToList(gvimVersionsSet)
	neovimVersions := setToList(neovimVersionsSet)

	sort.Slice(goVersions, func(i, j int) bool {
		lhs := strings.ReplaceAll(goVersions[i], "go", "v")
		rhs := strings.ReplaceAll(goVersions[j], "go", "v")
		return semver.Compare(lhs, rhs) < 0
	})
	sort.Slice(vimVersions, func(i, j int) bool {
		return vimSemverCompare(vimVersions[i], vimVersions[j]) < 0
	})
	sort.Slice(gvimVersions, func(i, j int) bool {
		return vimSemverCompare(gvimVersions[i], gvimVersions[j]) < 0
	})
	sort.Slice(neovimVersions, func(i, j int) bool {
		return semver.Compare(neovimVersions[i], neovimVersions[j]) < 0
	})

	if len(vimVersions) == 0 {
		panic(fmt.Errorf("found no vim versions"))
	}
	vs.MaxVimVersion = vimVersions[len(vimVersions)-1]
	vs.MaxGvimVersion = gvimVersions[len(gvimVersions)-1]
	vs.MaxNeovimVersion = neovimVersions[len(neovimVersions)-1]
	vs.GoVersions = strings.Join(goVersions, " ")
	vs.VimVersions = strings.Join(vimVersions, " ")
	vs.GvimVersions = strings.Join(gvimVersions, " ")
	vs.NeovimVersions = strings.Join(neovimVersions, " ")

	var flavStrings []string
	for _, f := range govim.Flavors {
		flavStrings = append(flavStrings, f.String())
	}
	vs.ValidFlavors = strings.Join(flavStrings, " ")
	writeFileFromTmpl(filepath.Join("_scripts", "gen_maxVersions_genconfig.bash"), maxVersions, vs)
}

// vimSemverCompare compares two Vim versions. Vim incorrectly puts leading
// zeroes on its versions, which means they are not semver.
func vimSemverCompare(i, j string) int {
	nonvi := i[1:]
	nonvj := j[1:]
	lhs := strings.Split(nonvi, ".")
	rhs := strings.Split(nonvj, ".")
	for i := 0; i < 3; i++ {
		lhs[i] = strings.TrimLeft(lhs[i], "0")
		rhs[i] = strings.TrimLeft(rhs[i], "0")
	}
	lhsv := fmt.Sprintf("v%v", strings.Join(lhs, "."))
	rhsv := fmt.Sprintf("v%v", strings.Join(rhs, "."))
	return semver.Compare(lhsv, rhsv)
}

// writeTravisYml assumes and writes a simple MxN matrix of Go versions and Vim versions
func writeGithubActionsYaml() {
	seenGoVersions := make(map[string]bool)
	seenVimFlavors := make(map[govim.Flavor]bool)
	seenVimVersions := make(map[string]bool)
	var goVersions []string
	var vimFlavors []string
	var vimVersions []string
	for _, b := range buildmatrix() {
		// TODO when we add Neovim to the actually build, drop this skip
		if b.vimflavor == govim.FlavorNeovim {
			continue
		}
		if !seenGoVersions[b.goversion] {
			seenGoVersions[b.goversion] = true
			goVersions = append(goVersions, b.goversion)
		}
		if !seenVimFlavors[b.vimflavor] {
			seenVimFlavors[b.vimflavor] = true
			vimFlavors = append(vimFlavors, b.vimflavor.String())
		}
		if !seenVimVersions[b.vimversion] {
			seenVimVersions[b.vimversion] = true
			vimVersions = append(vimVersions, b.vimversion)
		}
	}
	var entries = struct {
		GoVersions  string
		VimFlavors  string
		VimVersions string
	}{
		GoVersions:  stringSliceToString(goVersions),
		VimFlavors:  stringSliceToString(vimFlavors),
		VimVersions: stringSliceToString(vimVersions),
	}
	writeFileFromTmpl(".github/workflows/test.yml", travisYml, entries)
}

func stringSliceToString(s []string) string {
	var vs []string
	for _, v := range s {
		vs = append(vs, fmt.Sprintf("%q", v))
	}
	return "[" + strings.Join(vs, ", ") + "]"
}

func writeFileFromTmpl(path string, tmpl string, v interface{}) {
	fi, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	t := template.New(path)
	t.Delims("{{{", "}}}")
	t = template.Must(t.Parse(tmpl))
	if err := t.Execute(fi, v); err != nil {
		panic(err)
	}
	if err := fi.Close(); err != nil {
		panic(err)
	}
}

const travisYml = `# Code generated by genconfig. DO NOT EDIT.
on:
  push:
    branches:
      - master
  pull_request:
    branches:
      - '**'
  schedule:
    - cron: '0 9 * * *'

# actions/upload-artifact does not os.ExpandEnv the value passed
# to it as a path. Hence we have to hard code a directory that
# exists outside of the code checkout directory, and set ARTEFACTS
# accordingly below.
#
# Tracking https://github.com/actions/upload-artifact/issues/8

env:
  GO111MODULE: "on"
  GOPROXY: "https://proxy.golang.org"
  ARTEFACTS: "/home/runner/.artefacts"
  CI: "true"
  DOCKER_HUB_USER: "govimci"
  DOCKER_HUB_TOKEN: ${{ secrets.DOCKER_HUB_TOKEN }}
  GH_USER: "x-access-token"
  GH_TOKEN: ${{ github.token }}
  GOVIM_TEST_RACE_SLOWDOWN: "1.5"
  RACE_BUILD: 786

name: Go
jobs:
  test:
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest]
        go_version: {{{ .GoVersions }}}
        vim_flavor: {{{ .VimFlavors }}}
        vim_version: {{{ .VimVersions }}}
    runs-on: ${{ matrix.os }}
    env:
      GO_VERSION: ${{ matrix.go_version }}
      VIM_FLAVOR: ${{ matrix.vim_flavor }}
      VIM_VERSION: ${{ matrix.vim_version }}
    steps:
    - name: Checkout code
      uses: actions/checkout@722adc63f1aa60a57ec37892e133b1d319cae598
    - name: Build docker image
      run: ./_scripts/buildGovimImage.sh
    - name: Run Docker, run!
      if: success()
      run: ./_scripts/runDockerRun.sh
    - name: Tidy up
      if: success() || failure()
      run: ./_scripts/postRun.sh
    - name: Upload artefacts
      if: (success() || failure()) && env.CI_UPLOAD_ARTIFACTS == 'true'
      uses: actions/upload-artifact@3446296876d12d4e3a0f3145a3c87e67bf0a16b5
      with:
        path: /home/runner/.artefacts
        name: ${{ matrix.os }}_${{ matrix.go_version }}_${{ matrix.vim_flavor }}_${{ matrix.vim_version }}

  test-macos:
    strategy:
      fail-fast: false
      matrix:
        os: [macos-latest]
        go_version: ['1.14.x']
        vim_version: ["master"]
    runs-on: ${{ matrix.os }}
    env:
      VIM_FLAVOR: vim
    steps:
    - name: Checkout code
      uses: actions/checkout@722adc63f1aa60a57ec37892e133b1d319cae598
    - name: Install Vim
      uses: ./github/actions/setupvim
      with:
        version: ${{ matrix.vim_version }}
    - name: Install Go
      uses: actions/setup-go@9fbc767707c286e568c92927bbf57d76b73e0892
      with:
        go-version: ${{ matrix.go_version }}
    - name: Vim version
      run: vim --version
    - name: Go version
      run: go version
    - name: Run tests
      run: go run ./internal/cmd/dots go test -timeout 30m ./...
`

const maxVersions = `# Code generated by genconfig. DO NOT EDIT.
export GO_VERSIONS="{{{.GoVersions}}}"
export VIM_VERSIONS="{{{.VimVersions}}}"
export GVIM_VERSIONS="{{{.GvimVersions}}}"
export NEOVIM_VERSIONS="{{{.NeovimVersions}}}"

export MAX_GO_VERSION={{{.MaxGoVersion}}}
export MAX_VIM_VERSION={{{.MaxVimVersion}}}
export MAX_GVIM_VERSION={{{.MaxGvimVersion}}}
export MAX_NEOVIM_VERSION={{{.MaxNeovimVersion}}}

export DEFAULT_VIM_COMMAND={{{.VimCommand}}}
export DEFAULT_GVIM_COMMAND={{{.GvimCommand}}}
export DEFAULT_NEOVIM_COMMAND={{{.NeovimCommand}}}

export VALID_FLAVORS="{{{.ValidFlavors}}}"
`

func setToList(m map[string]bool) []string {
	var res []string
	for k := range m {
		res = append(res, k)
	}
	return res
}
