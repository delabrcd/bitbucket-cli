package api

import (
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
	err := checkNativeCommand(http.MethodPut, "/repositories/ws/repo/pullrequests/41/comments/12")
	if err == nil {
		t.Fatal("checkNativeCommand refused nothing for a covered write, want an error")
	}
	if !strings.Contains(err.Error(), "bb pr comment update") {
		t.Errorf("error = %q, want it to name the dedicated command", err.Error())
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %q, want it to mention --force", err.Error())
	}

	if err := checkNativeCommand(http.MethodGet, "/repositories/ws/repo/pullrequests"); err != nil {
		t.Errorf("checkNativeCommand refused a covered read: %v", err)
	}

	apiOptions.Force = true
	if err := checkNativeCommand(http.MethodPut, "/repositories/ws/repo/pullrequests/41/comments/12"); err != nil {
		t.Errorf("checkNativeCommand refused a covered write with --force: %v", err)
	}
}
