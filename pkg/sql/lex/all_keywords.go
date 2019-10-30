// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

// +build all-keywords

// all-keywords generates sql/lex/keywords.go from sql.y.
//
// It is generically structured with Go templates to allow for quick
// prototyping of different code generation structures for keyword token
// lookup. Previous attempts:
//
// Using github.com/cespare/mph to generate a perfect hash function. Was 10%
// slower. Also attempted to populate the mph.Table with a sparse array where
// the index correlated to the token id. This generated such a large array
// (~65k entries) that the mph package never returned from its Build call.
//
// A `KeywordsTokens = map[string]int32` map from string -> token id.
package main

import (
	"bufio"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"
)

func main() {
	blockRE := regexp.MustCompile(`^.*_keyword:`)
	keywordRE := regexp.MustCompile(`[A-Z].*`)

	// keyword indicates whether we are currently in a block prefixed by blockRE.
	keyword := false
	category := ""
	scanner := bufio.NewScanner(os.Stdin)
	type entry struct {
		Keyword, Ident, Category string
	}
	var data []entry
	// Look for lines that start with "XXX_keyword:" and record the category. For
	// subsequent non-empty lines, all words are keywords so add them to our
	// data list. An empty line indicates the end of the keyword section, so
	// stop recording.
	for scanner.Scan() {
		line := scanner.Text()
		if match := blockRE.FindString(line); match != "" {
			keyword = true
			category = categories[match]
			if category == "" {
				log.Fatal("unknown keyword type:", match)
			}
		} else if line == "" {
			keyword = false
		} else if match = keywordRE.FindString(line); keyword && match != "" {
			data = append(data, entry{
				Keyword:  strings.ToLower(match),
				Ident:    match,
				Category: category,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal("reading standard input:", err)
	}

	// Some output variables need their output to be sorted for deterministic
	// output.
	sort.Slice(data, func(i, j int) bool {
		return data[i].Ident < data[j].Ident
	})

	// Just panic if the template isn't parseable.
	if err := template.Must(template.New("").Parse(tmpl)).Execute(os.Stdout, data); err != nil {
		log.Fatal(err)
	}
}

// Category codes are for pg_get_keywords, see
// src/backend/utils/adt/misc.c in pg's sources.
var categories = map[string]string{
	"col_name_keyword:":                         "C",
	"unreserved_keyword:":                       "U",
	"type_func_name_keyword:":                   "T",
	"cockroachdb_extra_type_func_name_keyword:": "T",
	"reserved_keyword:":                         "R",
	"cockroachdb_extra_reserved_keyword:":       "R",
}

const tmpl = `// Code generated by cmd/all-keywords. DO NOT EDIT.

package lex

var KeywordsCategories = map[string]string{
{{range . -}}
	"{{.Keyword}}": "{{.Category}}",
{{end -}}
}

// KeywordNames contains all keywords sorted, so that pg_get_keywords returns
// deterministic results.
var KeywordNames = []string{
{{range . -}}
	"{{.Keyword}}",
{{end -}}
}

// GetKeywordID returns the lex id of the SQL keyword k or IDENT if k is
// not a keyword.
func GetKeywordID(k string) int32 {
	// The previous implementation generated a map that did a string ->
	// id lookup. Various ideas were benchmarked and the implementation below
	// was the fastest of those, between 3% and 10% faster (at parsing, so the
	// scanning speedup is even more) than the map implementation.
	switch k {
	{{range . -}}
	case "{{.Keyword}}": return {{.Ident}}
	{{end -}}
	default: return IDENT
	}
}
`
