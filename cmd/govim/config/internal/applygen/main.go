// applygen is a command that automates the generation of an Apply method on
// the pointer receiver of a struct type which has exported pointer-type fields
// to apply overrides from the argument onto the receiver
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"go/types"
	"os"
	"os/exec"
	"sort"

	"golang.org/x/tools/go/packages"
)

func main() {
	os.Exit(main1())
}

func main1() int {
	if err := mainerr(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ExitCode(ee.ProcessState)
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func mainerr() error {
	flag.Parse()

	if len(flag.Args()) == 0 {
		return fmt.Errorf("expected at least one argument")
	}

	cfg := &packages.Config{
		Mode: packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedName,
	}
	pkgs, err := packages.Load(cfg)
	if err != nil {
		return fmt.Errorf("failed to load types: %v", err)
	}
	if l := len(pkgs); l != 1 {
		return fmt.Errorf("expected a single package; got %v", l)
	}
	toGen := make(map[string][]string)
	var names []string
	pkg := pkgs[0]
	for _, name := range flag.Args() {
		names = append(names, name)
		tn := pkg.Types.Scope().Lookup(name)
		if tn == nil {
			return fmt.Errorf("failed to find type %v", name)
		}
		t := tn.(*types.TypeName).Type().Underlying()
		st := t.(*types.Struct)
		fields := make([]string, st.NumFields())
		for i := 0; i < st.NumFields(); i++ {
			fields[i] = st.Field(i).Name()
		}
		toGen[name] = fields
	}
	sort.Strings(names)

	var buf bytes.Buffer

	pf := func(format string, args ...interface{}) {
		if format[len(format)-1] != '\n' {
			format += "\n"
		}
		fmt.Fprintf(&buf, format, args...)
	}
	pf("package %v", pkg.Name)
	for _, name := range names {
		pf("func (r *%[1]v) Apply(v *%[1]v) {", name)
		for _, field := range toGen[name] {
			pf("if v.%[1]v != nil {", field)
			pf("  r.%[1]v = v.%[1]v", field)
			pf("}")
		}
		pf("}")
	}
	res, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format result: %v\n%s", err, buf.Bytes())
	}
	outFileName := "gen_applygen.go"
	outFile, err := os.Create("gen_applygen.go")
	if err != nil {
		return fmt.Errorf("failed to create %v: %v", outFileName, err)
	}
	if _, err := outFile.Write(res); err != nil {
		return fmt.Errorf("failed to write to %v: %v", outFileName, err)
	}
	if err := outFile.Close(); err != nil {
		return fmt.Errorf("failed to close %v: %v", outFileName, err)
	}

	return nil
}
