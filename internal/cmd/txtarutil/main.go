// txtarutil manipulates the "footer" of a txtar archive.
//
// Usage:
//
//    txtarutil (add|drop)footer [-unless PATTERN] GLOB [FILE]
//
// where GLOB is a pattern per path/filepath.Glob. If FILE is not supplied for
// those commands where one is required, then stdin is read for the content. add
// has the semantics of ensuring a footer is in place, rather than blindly adding
// the same footer. If -unless is supplied, PATTERN is interpretted as a regular
// expression to be matched against each line of an archive's comment; if for a
// given archive matching GLOB the PATTERN matches, that archive is skipped.
//
// For example:
//
// 	txtarutil (add|drop)footer ./cmd/govim/testdata/scenario_*/*.txt footer.txt
//
// Or using a heredoc:
//
// 	txtarutil addfooter ./cmd/govim/testdata/scenario_*/*.txt << EOD
//
// 	# Common footer
// 	exec echo Hello, World!
//
// 	EOD
//
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"unicode"

	"github.com/rogpeppe/go-internal/txtar"
)

var (
	flagSet = flag.NewFlagSet("txtarutil", flag.ContinueOnError)
)

func init() { flagSet.Usage = usage }

func usage() {
	fmt.Fprintf(os.Stderr, `
Usage:

   txtarutil (add|drop)footer [-unless PATTERN] GLOB [FILE]

where GLOB is a pattern per path/filepath.Glob. If FILE is not supplied for
those commands where one is required, then stdin is read for the content. add
has the semantics of ensuring a footer is in place, rather than blindly adding
the same footer. If -unless is supplied, PATTERN is interpretted as a regular
expression to be matched against each line of an archive's comment; if for a
given archive matching GLOB the PATTERN matches, that archive is skipped.

For example:

	txtarutil (add|drop)footer ./cmd/govim/testdata/scenario_*/*.txt footer.txt

Or using a heredoc:

	txtarutil addfooter ./cmd/govim/testdata/scenario_*/*.txt << EOD

	# Common footer
	exec echo Hello, World!

	EOD

`[1:])
	flagSet.PrintDefaults()
}

type usageErr string

func (u usageErr) Error() string { return string(u) }

type flagErr string

func (f flagErr) Error() string { return string(f) }

func main() { os.Exit(main1()) }

func main1() int {
	err := mainerr()
	if err == nil {
		return 0
	}
	switch err.(type) {
	case usageErr:
		fmt.Fprintln(os.Stderr, err)
		flagSet.Usage()
		return 2
	case flagErr:
		return 2
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

func mainerr() error {
	if err := flagSet.Parse(os.Args[1:]); err != nil {
		return flagErr(err.Error())
	}
	args := flagSet.Args()
	if len(args) == 0 {
		return usageErr("need a command")
	}
	handlers := map[string]func([]string) error{
		"addfooter":  addfooter,
		"dropfooter": dropfooter,
	}
	command := handlers[args[0]]
	if command == nil {
		return usageErr(fmt.Sprintf("unknown command %q", args[0]))
	}
	return command(args[1:])
}

func dropfooter(args []string) error {
	return transformEachMatchWithFooter("addfooter", args, dropFooterImpl)
}

func addfooter(args []string) error {
	return transformEachMatchWithFooter("addfooter", args, dropFooterImpl, addFooterImpl)
}

func dropFooterImpl(archive *txtar.Archive, footer []byte) {
	archive.Comment = append([]byte(nil), archive.Comment...)
	archive.Comment = bytes.TrimRightFunc(archive.Comment, unicode.IsSpace)
	archive.Comment = bytes.TrimSuffix(archive.Comment, bytes.TrimSpace(footer))
	archive.Comment = bytes.TrimRightFunc(archive.Comment, unicode.IsSpace)
	if len(archive.Comment) > 0 {
		archive.Comment = append(archive.Comment, '\n')
	}
}

func addFooterImpl(archive *txtar.Archive, footer []byte) {
	archive.Comment = append(archive.Comment, footer...)
}

func transformEachMatchWithFooter(fn string, args []string, txfms ...func(archive *txtar.Archive, footer []byte)) error {
	if len(txfms) == 0 {
		return nil
	}
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fUnless := fs.String("unless", "", "regular expression that determines when not to apply transform")
	if err := fs.Parse(args); err != nil {
		return usageErr(fn + ": " + err.Error())
	}
	args = fs.Args()
	switch len(args) {
	case 1, 2:
	default:
		return usageErr(fn + ": incorrect number of arguments")
	}
	errf := func(format string, args ...interface{}) error {
		return fmt.Errorf(fn+": "+format, args...)
	}
	var unless *regexp.Regexp
	if *fUnless != "" {
		r, err := regexp.Compile(*fUnless)
		if err != nil {
			return errf("failed to parse regular expression %q: %v", *fUnless, err)
		}
		unless = r
	}
	glob := args[0]
	matches, err := filepath.Glob(glob)
	if err != nil {
		return errf("bad glob %q: %v\n", glob, err)
	}
	var footer []byte
	if len(args) == 2 {
		fn := args[1]
		contents, err := ioutil.ReadFile(fn)
		if err != nil {
			return errf("failed to load %v: %v", fn, err)
		}
		footer = contents
	} else {
		contents, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return errf("failed to read from stdin: %v", err)
		}
		footer = contents
	}
Matches:
	for _, m := range matches {
		f, err := ioutil.ReadFile(m)
		if err != nil {
			return errf("failed to read %v: %v", f, err)
		}
		archive := txtar.Parse(f)
		if unless != nil {
			for _, line := range bytes.Split(archive.Comment, []byte("\n")) {
				if unless.Match(line) {
					continue Matches
				}
			}
		}
		for _, t := range txfms {
			t(archive, footer)
		}
		var b bytes.Buffer
		fmt.Fprintf(&b, "%s", archive.Comment)
		if len(archive.Comment) > 0 && len(archive.Files) > 0 {
			fmt.Fprintf(&b, "\n")
		}
		for _, f := range archive.Files {
			fmt.Fprintf(&b, "-- %v --\n%s", f.Name, f.Data)
		}
		if err := ioutil.WriteFile(m, b.Bytes(), 0666); err != nil {
			return fmt.Errorf("failed to write back to %v: %v", m, err)
		}
	}
	return nil

}
