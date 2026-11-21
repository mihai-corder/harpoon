package config

import (
	"fmt"
	"sort"
	"strings"
)

// Select returns a new collection holding only the named requests, in the
// collection's own order. It is shared by the CLI (--request) and the web
// API, so both behave identically.
func Select(coll *Collection, names []string) (*Collection, error) {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}

	out := &Collection{Name: coll.Name, Vars: coll.Vars}
	for _, nr := range coll.Requests {
		if want[nr.Name] {
			out.Requests = append(out.Requests, nr)
			delete(want, nr.Name)
		}
	}

	if len(want) > 0 {
		unknown := make([]string, 0, len(want))
		for n := range want {
			unknown = append(unknown, n)
		}
		sort.Strings(unknown)

		available := make([]string, 0, len(coll.Requests))
		for _, nr := range coll.Requests {
			available = append(available, nr.Name)
		}
		return nil, fmt.Errorf(
			"unknown request(s): %s (available: %s)",
			strings.Join(unknown, ", "), strings.Join(available, ", "))
	}
	return out, nil
}
