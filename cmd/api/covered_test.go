package api

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestTrimAPIRoot(t *testing.T) {
	cases := map[string]string{
		"repositories/ws/repo":                                         "repositories/ws/repo",
		"/repositories/ws/repo":                                        "repositories/ws/repo",
		"/2.0/repositories/ws/repo":                                    "repositories/ws/repo",
		"https://api.bitbucket.org/2.0/repositories/ws/repo":           "repositories/ws/repo",
		"https://api.bitbucket.org/2.0/user":                           "user",
		"repositories/ws/repo/pullrequests?state=OPEN&q=title~%22x%22": "repositories/ws/repo/pullrequests",
		"/2.0": "",
	}
	for in, want := range cases {
		if got := trimAPIRoot(in); got != want {
			t.Errorf("trimAPIRoot(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchNativeRouteFindsCoveredEndpoints(t *testing.T) {
	cases := []struct {
		method   string
		endpoint string
		command  string
	}{
		{http.MethodPut, "/repositories/ws/repo/pullrequests/41/comments/831989877", "bb pr comment update"},
		{http.MethodPost, "/repositories/ws/repo/pullrequests/41/comments", "bb pr comment create"},
		{http.MethodPost, "/repositories/ws/repo/pullrequests/41/comments/12/resolve", "bb pr comment resolve"},
		{http.MethodDelete, "/repositories/ws/repo/pullrequests/41/comments/12/resolve", "bb pr comment reopen"},
		{http.MethodGet, "/repositories/{workspace}/{repo}/pullrequests", "bb pr list"},
		{http.MethodPost, "/repositories/ws/repo/pullrequests", "bb pr create"},
		{http.MethodPut, "/repositories/ws/repo/pullrequests/41", "bb pr update"},
		{http.MethodPost, "/repositories/ws/repo/pullrequests/41/merge", "bb pr merge"},
		{http.MethodDelete, "/repositories/ws/repo/pullrequests/41/approve", "bb pr unapprove"},
		{http.MethodPut, "/repositories/ws/repo/issues/7/comments/9", "bb issue comment update"},
		{http.MethodGet, "https://api.bitbucket.org/2.0/repositories/ws/repo/refs/branches", "bb branch list"},
		{http.MethodDelete, "/repositories/ws/repo/refs/tags/release/v1.2.3", "bb tag delete"},
		{http.MethodGet, "/repositories/ws/repo/downloads/dir/artifact.tar.gz", "bb artifact download"},
		{http.MethodDelete, "/workspaces/ws/pipelines-config/runners/{uuid}", "bb runner delete <uuid> --workspace-level"},
		{http.MethodGet, "/user", "bb user me"},
		{http.MethodGet, "/users/uuid/ssh-keys", "bb ssh-key list"},
	}
	for _, c := range cases {
		route := matchNativeRoute(c.method, c.endpoint)
		if route == nil {
			t.Errorf("matchNativeRoute(%s %s) = nil, want %q", c.method, c.endpoint, c.command)
			continue
		}
		if !strings.Contains(route.Command, c.command) {
			t.Errorf("matchNativeRoute(%s %s) = %q, want it to contain %q", c.method, c.endpoint, route.Command, c.command)
		}
	}
}

func TestMatchNativeRouteLeavesUncoveredEndpointsAlone(t *testing.T) {
	cases := []struct {
		method   string
		endpoint string
	}{
		{http.MethodGet, "/repositories/ws/repo/deployments"},
		{http.MethodPut, "/repositories/ws/repo/branch-restrictions/42"},
		{http.MethodPost, "/repositories/ws/repo/commit/abc123/statuses/build"},
		{http.MethodGet, "/repositories/ws/repo/pullrequests/41/comments/12/foo"},
		{http.MethodPost, "/repositories/ws/repo/refs/branches"}, // creating a branch has no bb command
		{http.MethodPatch, "/repositories/ws/repo/pullrequests/41"},
		{http.MethodGet, "/"},
	}
	for _, c := range cases {
		if route := matchNativeRoute(c.method, c.endpoint); route != nil {
			t.Errorf("matchNativeRoute(%s %s) = %q, want nil", c.method, c.endpoint, route.Command)
		}
	}
}

func TestCheckNativeCommandRefusesWritesAndAllowsReads(t *testing.T) {
	defer func() { apiOptions.Force = false }()

	apiOptions.Force = false
	err := checkNativeCommand(http.MethodPut, "/repositories/ws/repo/pullrequests/41/comments/12", nil)
	if err == nil {
		t.Fatal("checkNativeCommand refused nothing for a covered write, want an error")
	}
	if !strings.Contains(err.Error(), "bb pr comment update") {
		t.Errorf("error = %q, want it to name the dedicated command", err.Error())
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %q, want it to mention --force", err.Error())
	}

	if err := checkNativeCommand(http.MethodGet, "/repositories/ws/repo/pullrequests", nil); err != nil {
		t.Errorf("checkNativeCommand refused a covered read: %v", err)
	}

	apiOptions.Force = true
	if err := checkNativeCommand(http.MethodPut, "/repositories/ws/repo/pullrequests/41/comments/12", nil); err != nil {
		t.Errorf("checkNativeCommand refused a covered write with --force: %v", err)
	}
}

func TestCheckNativeCommandAllowsBodyTheCommandCannotExpress(t *testing.T) {
	defer func() { apiOptions.Force = false }()
	apiOptions.Force = false

	if err := checkNativeCommand(http.MethodPut, "/repositories/ws/repo/pullrequests/186", []string{"draft"}); err == nil {
		t.Error("checkNativeCommand allowed a body bb pr update can express, want a refusal")
	}

	if err := checkNativeCommand(http.MethodPut, "/repositories/ws/repo/pullrequests/186", []string{"draft", "merge_strategy"}); err != nil {
		t.Errorf("checkNativeCommand refused a body bb pr update cannot express: %v", err)
	}

	if err := checkNativeCommand(http.MethodPut, "/repositories/ws/repo/pullrequests/41/tasks/7", []string{"anything"}); err == nil {
		t.Error("checkNativeCommand allowed a write on a route with no body contract, want a refusal")
	}
}

func TestCheckNativeCommandSurfacesTheInlineCaveat(t *testing.T) {
	defer func() { apiOptions.Force = false }()
	apiOptions.Force = false

	err := checkNativeCommand(http.MethodPut, "/repositories/ws/repo/pullrequests/41/comments/12", []string{"content"})
	if err == nil {
		t.Fatal("checkNativeCommand allowed a covered comment update, want a refusal")
	}
	if !strings.Contains(err.Error(), "inline") {
		t.Errorf("error = %q, want it to mention the inline caveat", err.Error())
	}
}

// bodyContracts keys must keep matching an entry in nativeRoutes.
func TestBodyContractsMatchRoutes(t *testing.T) {
	known := map[string]bool{}
	for _, route := range nativeRoutes {
		for _, method := range route.Methods {
			known[method+" "+route.Pattern] = true
		}
	}
	for key := range bodyContracts {
		if !known[key] {
			t.Errorf("bodyContracts has %q, which matches no entry in nativeRoutes", key)
		}
	}
}

func TestQuoteAndJoin(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"a"}, `"a"`},
		{[]string{"a", "b"}, `"a" and "b"`},
		{[]string{"a", "b", "c"}, `"a", "b" and "c"`},
	}
	for _, tc := range cases {
		if got := quoteAndJoin(tc.in); got != tc.want {
			t.Errorf("quoteAndJoin(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBodyTopLevelKeys(t *testing.T) {
	if got := bodyTopLevelKeys(nil); got != nil {
		t.Errorf("bodyTopLevelKeys(nil) = %v, want nil", got)
	}

	fields := map[string]interface{}{"title": "x", "source.branch.name": "feature/y", "draft": true}
	want := []string{"draft", "source", "title"}
	got := bodyTopLevelKeys(fields)
	if len(got) != len(want) {
		t.Fatalf("bodyTopLevelKeys(fields) = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("bodyTopLevelKeys(fields) = %v, want %v", got, want)
		}
	}
}

func TestBodyTopLevelKeysFromReaderLeavesBodyIntact(t *testing.T) {
	raw := []byte(`{"draft": false, "title": "ready"}`)
	reader := bytes.NewReader(raw)

	keys := bodyTopLevelKeys(reader)
	if len(keys) != 2 || keys[0] != "draft" || keys[1] != "title" {
		t.Errorf("bodyTopLevelKeys(reader) = %v, want [draft title]", keys)
	}

	sent, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading the body after inspection: %v", err)
	}
	if string(sent) != string(raw) {
		t.Errorf("body after inspection = %q, want %q", sent, raw)
	}
}
