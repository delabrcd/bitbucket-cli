package pullrequest

import "testing"

func TestComposeStateQueryFoldsStateIntoQuery(t *testing.T) {
	cases := []struct {
		name  string
		query string
		state string
		want  string
	}{
		{"state folded into query", `title~"fix"`, "open", `(title~"fix") AND state="OPEN"`},
		{"state uppercased", `title~"fix"`, "declined", `(title~"fix") AND state="DECLINED"`},
		{"all means no predicate", `title~"fix"`, "all", `title~"fix"`},
		{"empty state means no predicate", `title~"fix"`, "", `title~"fix"`},
		{"query alone becomes a state predicate", "", "open", `state="OPEN"`},
		{"neither yields empty", "", "all", ""},
		{"existing OR is parenthesised", `title~"a" OR title~"b"`, "open", `(title~"a" OR title~"b") AND state="OPEN"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := composeStateQuery(tc.query, tc.state); got != tc.want {
				t.Errorf("composeStateQuery(%q, %q) = %q, want %q", tc.query, tc.state, got, tc.want)
			}
		})
	}
}

func TestListStateFlagDefaultsToOpen(t *testing.T) {
	if got := listOptions.State.String(); got != "open" {
		t.Errorf("default --state = %q, want %q", got, "open")
	}
}
