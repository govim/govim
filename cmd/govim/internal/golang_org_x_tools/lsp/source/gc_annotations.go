// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
)

func GCOptimizationDetails(ctx context.Context, snapshot Snapshot, pkgDir span.URI) (map[VersionedFileIdentity][]*Diagnostic, error) {
	outDir := filepath.Join(os.TempDir(), fmt.Sprintf("gopls-%d.details", os.Getpid()))
	if err := os.MkdirAll(outDir, 0700); err != nil {
		return nil, err
	}
	args := []string{fmt.Sprintf("-gcflags=-json=0,%s", outDir),
		fmt.Sprintf("-o=%s", pkgDir.Filename()),
		pkgDir.Filename(),
	}
	err := snapshot.RunGoCommandDirect(ctx, "build", args)
	if err != nil {
		return nil, err
	}
	files, err := findJSONFiles(outDir)
	if err != nil {
		return nil, err
	}
	reports := make(map[VersionedFileIdentity][]*Diagnostic)
	opts := snapshot.View().Options()
	var parseError error
	for _, fn := range files {
		fname, v, err := parseDetailsFile(fn)
		if err != nil {
			// expect errors for all the files, save 1
			parseError = err
		}
		if !strings.HasSuffix(fname, ".go") {
			continue // <autogenerated>
		}
		uri := span.URIFromPath(fname)
		x := snapshot.FindFile(uri)
		if x == nil {
			continue
		}
		v = filterDiagnostics(v, &opts)
		reports[x.VersionedFileIdentity()] = v
	}
	return reports, parseError
}

func filterDiagnostics(v []*Diagnostic, o *Options) []*Diagnostic {
	var ans []*Diagnostic
	for _, x := range v {
		if x.Source != "go compiler" {
			continue
		}
		if o.Annotations["noInline"] &&
			(strings.HasPrefix(x.Message, "canInline") ||
				strings.HasPrefix(x.Message, "cannotInline") ||
				strings.HasPrefix(x.Message, "inlineCall")) {
			continue
		} else if o.Annotations["noEscape"] &&
			(strings.HasPrefix(x.Message, "escape") || x.Message == "leak") {
			continue
		} else if o.Annotations["noNilcheck"] && strings.HasPrefix(x.Message, "nilcheck") {
			continue
		} else if o.Annotations["noBounds"] &&
			(strings.HasPrefix(x.Message, "isInBounds") ||
				strings.HasPrefix(x.Message, "isSliceInBounds")) {
			continue
		}
		ans = append(ans, x)
	}
	return ans
}

func parseDetailsFile(fn string) (string, []*Diagnostic, error) {
	buf, err := ioutil.ReadFile(fn)
	if err != nil {
		return "", nil, err // This is an internal error. Likely ever file will fail.
	}
	var fname string
	var ans []*Diagnostic
	lines := bytes.Split(buf, []byte{'\n'})
	for i, l := range lines {
		if len(l) == 0 {
			continue
		}
		if i == 0 {
			x := make(map[string]interface{})
			if err := json.Unmarshal(l, &x); err != nil {
				return "", nil, fmt.Errorf("internal error (%v) parsing first line of json file %s",
					err, fn)
			}
			fname = x["file"].(string)
			continue
		}
		y := protocol.Diagnostic{}
		if err := json.Unmarshal(l, &y); err != nil {
			return "", nil, fmt.Errorf("internal error (%#v) parsing json file for %s", err, fname)
		}
		y.Range.Start.Line-- // change from 1-based to 0-based
		y.Range.Start.Character--
		y.Range.End.Line--
		y.Range.End.Character--
		msg := y.Code.(string)
		if y.Message != "" {
			msg = fmt.Sprintf("%s(%s)", msg, y.Message)
		}
		x := Diagnostic{
			Range:    y.Range,
			Message:  msg,
			Source:   y.Source,
			Severity: y.Severity,
		}
		for _, ri := range y.RelatedInformation {
			x.Related = append(x.Related, RelatedInformation{
				URI:     ri.Location.URI.SpanURI(),
				Range:   ri.Location.Range,
				Message: ri.Message,
			})
		}
		ans = append(ans, &x)
	}
	return fname, ans, nil
}

func findJSONFiles(dir string) ([]string, error) {
	ans := []string{}
	f := func(path string, fi os.FileInfo, err error) error {
		if fi.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".json") {
			ans = append(ans, path)
		}
		return nil
	}
	err := filepath.Walk(dir, f)
	return ans, err
}
