package api

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/gildas/go-errors"
)

// nativeRoute ties a REST route to the bb command that already covers it.
//
// Dedicated commands do more than the raw request: they normalize markdown,
// validate anchors and flags, resolve the workspace/repository/pullrequest from
// the git context, and render the response through the shared output formatting.
// Going around them with "bb api" silently loses all of that, so the routes
// listed here are refused unless --force is given.
type nativeRoute struct {
	Methods []string // nil matches any method
	Pattern string   // path pattern: "*" matches one segment, "**" matches one or more trailing segments
	Command string   // the equivalent bb command
	Adds    string   // what the command does that the raw request does not (optional)
}

// bodyContract describes what a dedicated command can express in a request body.
// TestBodyContractsMatchRoutes keeps it in sync with nativeRoutes.
type bodyContract struct {
	// Expresses lists the top-level body keys the dedicated command can set.
	Expresses []string

	// Caveat is appended to a refusal, for a route where the raw request has a trap
	// the dedicated command handles.
	Caveat string
}

// bodyContracts is keyed by "<METHOD> <pattern>", matching an entry in nativeRoutes.
var bodyContracts = map[string]bodyContract{
	http.MethodPost + " repositories/*/*/pullrequests": {
		Expresses: []string{"title", "description", "summary", "source", "destination", "close_source_branch", "draft", "reviewers"},
	},
	http.MethodPut + " repositories/*/*/pullrequests/*": {
		Expresses: []string{"title", "description", "summary", "destination", "close_source_branch", "draft", "reviewers"},
	},
	http.MethodPost + " repositories/*/*/pullrequests/*/comments": {
		Expresses: []string{"content", "inline", "parent", "pending"},
	},
	http.MethodPut + " repositories/*/*/pullrequests/*/comments/*": {
		Expresses: []string{"content", "pending"},
		Caveat:    "Bitbucket rejects an \"inline\" key on a comment update, so no request can move the anchor.",
	},
	http.MethodPost + " repositories/*/*/issues": {
		Expresses: []string{"title", "content", "kind", "priority", "state", "assignee", "component", "milestone", "version"},
	},
	http.MethodPut + " repositories/*/*/issues/*": {
		Expresses: []string{"title", "content", "kind", "priority", "state", "assignee", "component", "milestone", "version"},
	},
}

// contract returns the body contract for a route, if one is defined.
func (route *nativeRoute) contract() (bodyContract, bool) {
	for _, method := range route.Methods {
		if contract, found := bodyContracts[method+" "+route.Pattern]; found {
			return contract, true
		}
	}
	return bodyContract{}, false
}

// unexpressedFields reports the body keys the dedicated command cannot set.
func (route *nativeRoute) unexpressedFields(bodyKeys []string) []string {
	contract, found := route.contract()
	if !found || len(bodyKeys) == 0 {
		return nil
	}

	var unexpressed []string
	for _, key := range bodyKeys {
		if !slices.Contains(contract.Expresses, key) {
			unexpressed = append(unexpressed, key)
		}
	}
	return unexpressed
}

// nativeRoutes is ordered: the first match wins, so specific patterns come
// before the generic ones they live under.
var nativeRoutes = []nativeRoute{
	// Pull request comments
	{[]string{http.MethodPost}, "repositories/*/*/pullrequests/*/comments/*/resolve", "bb pr comment resolve <comment-id> --pullrequest <id>", ""},
	{[]string{http.MethodDelete}, "repositories/*/*/pullrequests/*/comments/*/resolve", "bb pr comment reopen <comment-id> --pullrequest <id>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/pullrequests/*/comments", "bb pr comment list --pullrequest <id>", ""},
	{[]string{http.MethodPost}, "repositories/*/*/pullrequests/*/comments", "bb pr comment create --pullrequest <id> --comment <text>", "normalizes the markdown, anchors inline comments with --file/--line, and keeps the review pending with --pending"},
	{[]string{http.MethodGet}, "repositories/*/*/pullrequests/*/comments/*", "bb pr comment get <comment-id> --pullrequest <id>", ""},
	{[]string{http.MethodPut}, "repositories/*/*/pullrequests/*/comments/*", "bb pr comment update <comment-id> --pullrequest <id> --comment <text>", "normalizes the markdown and preserves the inline anchor and pending state"},
	{[]string{http.MethodDelete}, "repositories/*/*/pullrequests/*/comments/*", "bb pr comment delete <comment-id> --pullrequest <id>", ""},

	// Pull request tasks
	{[]string{http.MethodGet}, "repositories/*/*/pullrequests/*/tasks", "bb pr task list --pullrequest <id>", ""},
	{[]string{http.MethodPost}, "repositories/*/*/pullrequests/*/tasks", "bb pr task create --pullrequest <id>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/pullrequests/*/tasks/*", "bb pr task get <task-id> --pullrequest <id>", ""},
	{[]string{http.MethodPut}, "repositories/*/*/pullrequests/*/tasks/*", "bb pr task update <task-id> --pullrequest <id>", ""},
	{[]string{http.MethodDelete}, "repositories/*/*/pullrequests/*/tasks/*", "bb pr task delete <task-id> --pullrequest <id>", ""},

	// Pull request actions
	{[]string{http.MethodPost}, "repositories/*/*/pullrequests/*/approve", "bb pr approve <id>", ""},
	{[]string{http.MethodDelete}, "repositories/*/*/pullrequests/*/approve", "bb pr unapprove <id>", ""},
	{[]string{http.MethodPost}, "repositories/*/*/pullrequests/*/request-changes", "bb pr request-changes <id>", ""},
	{[]string{http.MethodDelete}, "repositories/*/*/pullrequests/*/request-changes", "bb pr remove-request-changes <id>", ""},
	{[]string{http.MethodPost}, "repositories/*/*/pullrequests/*/decline", "bb pr decline <id>", ""},
	{[]string{http.MethodPost}, "repositories/*/*/pullrequests/*/merge", "bb pr merge <id>", "verifies the merge checks first and can follow the async merge task"},
	{[]string{http.MethodGet}, "repositories/*/*/pullrequests/*/merge/task-status/*", "bb pr merge-status --task-id <task-id>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/pullrequests/*/diff", "bb pr diff <id>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/pullrequests/*/diffstat", "bb pr diff <id> --stat", ""},
	{[]string{http.MethodGet}, "repositories/*/*/pullrequests/*/patch", "bb pr patch <id>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/pullrequests/*/commits", "bb pr commits <id>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/pullrequests/*/activity", "bb pr activity list --pullrequest <id>", ""},

	// Pull requests
	{[]string{http.MethodGet}, "repositories/*/*/pullrequests", "bb pr list", ""},
	{[]string{http.MethodPost}, "repositories/*/*/pullrequests", "bb pr create", "normalizes the description markdown, pulls in the default reviewers, and can open the pullrequest as a draft"},
	{[]string{http.MethodGet}, "repositories/*/*/pullrequests/*", "bb pr get <id>", ""},
	{[]string{http.MethodPut}, "repositories/*/*/pullrequests/*", "bb pr update <id>", "normalizes the description markdown and leaves the fields you do not pass untouched"},
	{[]string{http.MethodGet}, "repositories/*/*/commit/*/pullrequests", "bb pr list --commit <commit>", ""},

	// Issue comments and attachments
	{[]string{http.MethodGet}, "repositories/*/*/issues/*/comments", "bb issue comment list --issue <id>", ""},
	{[]string{http.MethodPost}, "repositories/*/*/issues/*/comments", "bb issue comment create --issue <id> --comment <text>", "normalizes the markdown"},
	{[]string{http.MethodGet}, "repositories/*/*/issues/*/comments/*", "bb issue comment get <comment-id> --issue <id>", ""},
	{[]string{http.MethodPut}, "repositories/*/*/issues/*/comments/*", "bb issue comment update <comment-id> --issue <id> --comment <text>", "normalizes the markdown"},
	{[]string{http.MethodDelete}, "repositories/*/*/issues/*/comments/*", "bb issue comment delete <comment-id> --issue <id>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/issues/*/attachments", "bb issue attachment list --issue <id>", ""},
	{[]string{http.MethodPost}, "repositories/*/*/issues/*/attachments", "bb issue attachment upload --issue <id>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/issues/*/attachments/**", "bb issue attachment download <filename> --issue <id>", ""},
	{[]string{http.MethodDelete}, "repositories/*/*/issues/*/attachments/**", "bb issue attachment delete <filename> --issue <id>", ""},

	// Issue actions
	{[]string{http.MethodPut}, "repositories/*/*/issues/*/vote", "bb issue vote <id>", ""},
	{[]string{http.MethodDelete}, "repositories/*/*/issues/*/vote", "bb issue unvote <id>", ""},
	{[]string{http.MethodPut}, "repositories/*/*/issues/*/watch", "bb issue watch <id>", ""},
	{[]string{http.MethodDelete}, "repositories/*/*/issues/*/watch", "bb issue unwatch <id>", ""},

	// Issues
	{[]string{http.MethodGet}, "repositories/*/*/issues", "bb issue list", ""},
	{[]string{http.MethodPost}, "repositories/*/*/issues", "bb issue create", "normalizes the description markdown"},
	{[]string{http.MethodGet}, "repositories/*/*/issues/*", "bb issue get <id>", ""},
	{[]string{http.MethodPut}, "repositories/*/*/issues/*", "bb issue update <id>", "normalizes the description markdown"},
	{[]string{http.MethodDelete}, "repositories/*/*/issues/*", "bb issue delete <id>", ""},

	// Pipelines
	{[]string{http.MethodGet}, "repositories/*/*/pipelines/*/steps", "bb pipeline step list --pipeline <uuid-or-number>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/pipelines/*/steps/*", "bb pipeline step get <step-uuid> --pipeline <uuid-or-number>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/pipelines/*/steps/*/log", "bb pipeline logs <uuid-or-number> --step <step-uuid>", ""},
	{[]string{http.MethodPost}, "repositories/*/*/pipelines/*/stopPipeline", "bb pipeline stop <uuid-or-number>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/pipelines", "bb pipeline list", ""},
	{[]string{http.MethodPost}, "repositories/*/*/pipelines", "bb pipeline trigger", ""},
	{[]string{http.MethodGet}, "repositories/*/*/pipelines/*", "bb pipeline get <uuid-or-number>", ""},

	// Pipelines runners
	{[]string{http.MethodGet}, "repositories/*/*/pipelines-config/runners", "bb runner list", ""},
	{[]string{http.MethodPost}, "repositories/*/*/pipelines-config/runners", "bb runner create", ""},
	{[]string{http.MethodGet}, "repositories/*/*/pipelines-config/runners/*", "bb runner get <uuid>", ""},
	{[]string{http.MethodDelete}, "repositories/*/*/pipelines-config/runners/*", "bb runner delete <uuid>", ""},
	{[]string{http.MethodGet}, "workspaces/*/pipelines-config/runners", "bb runner list --workspace-level", ""},
	{[]string{http.MethodPost}, "workspaces/*/pipelines-config/runners", "bb runner create --workspace-level", ""},
	{[]string{http.MethodGet}, "workspaces/*/pipelines-config/runners/*", "bb runner get <uuid> --workspace-level", ""},
	{[]string{http.MethodDelete}, "workspaces/*/pipelines-config/runners/*", "bb runner delete <uuid> --workspace-level", ""},

	// Repository contents
	{[]string{http.MethodGet}, "repositories/*/*/refs/branches", "bb branch list", ""},
	{[]string{http.MethodGet}, "repositories/*/*/refs/tags", "bb tag list", ""},
	{[]string{http.MethodPost}, "repositories/*/*/refs/tags", "bb tag create --name <tag-name>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/refs/tags/**", "bb tag get <tag-name>", ""},
	{[]string{http.MethodDelete}, "repositories/*/*/refs/tags/**", "bb tag delete <tag-name>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/commits", "bb commit list", ""},
	{[]string{http.MethodGet}, "repositories/*/*/commit/*", "bb commit get <commit>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/diff/**", "bb commit diff <commit> [<commit>]", ""},
	{[]string{http.MethodGet}, "repositories/*/*/patch/**", "bb commit patch <commit> [<commit>]", ""},
	{[]string{http.MethodGet}, "repositories/*/*/components", "bb component list", ""},
	{[]string{http.MethodGet}, "repositories/*/*/components/*", "bb component get <component-id>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/downloads", "bb artifact list", ""},
	{[]string{http.MethodPost}, "repositories/*/*/downloads", "bb artifact upload <file>", ""},
	{[]string{http.MethodGet}, "repositories/*/*/downloads/**", "bb artifact download <filename>", ""},
	{[]string{http.MethodDelete}, "repositories/*/*/downloads/**", "bb artifact delete <filename>", ""},

	// Repositories
	{[]string{http.MethodGet}, "repositories/*/*/forks", "bb repo get <slug> --forks", ""},
	{[]string{http.MethodPost}, "repositories/*/*/forks", "bb repo fork <slug>", ""},
	{[]string{http.MethodGet}, "repositories/*/*", "bb repo get <slug>", ""},
	{[]string{http.MethodPost}, "repositories/*/*", "bb repo create <slug>", ""},
	{[]string{http.MethodPut}, "repositories/*/*", "bb repo update <slug>", ""},
	{[]string{http.MethodDelete}, "repositories/*/*", "bb repo delete <slug>", ""},
	{[]string{http.MethodGet}, "repositories/*", "bb repo list --workspace <workspace>", ""},
	{[]string{http.MethodGet}, "repositories", "bb repo list", ""},

	// Projects
	{[]string{http.MethodGet}, "workspaces/*/projects/*/default-reviewers", "bb project reviewer list --project <key>", ""},
	{[]string{http.MethodGet}, "workspaces/*/projects/*/default-reviewers/*", "bb project reviewer get <user> --project <key>", ""},
	{[]string{http.MethodPut}, "workspaces/*/projects/*/default-reviewers/*", "bb project reviewer add <user> --project <key>", ""},
	{[]string{http.MethodDelete}, "workspaces/*/projects/*/default-reviewers/*", "bb project reviewer delete <user> --project <key>", ""},
	{[]string{http.MethodGet}, "workspaces/*/projects", "bb project list", ""},
	{[]string{http.MethodPost}, "workspaces/*/projects", "bb project create", ""},
	{[]string{http.MethodGet}, "workspaces/*/projects/*", "bb project get <key>", ""},
	{[]string{http.MethodPut}, "workspaces/*/projects/*", "bb project update <key>", ""},
	{[]string{http.MethodDelete}, "workspaces/*/projects/*", "bb project delete <key>", ""},

	// Workspaces and users
	{[]string{http.MethodGet}, "workspaces/*/permissions", "bb workspace permission list <workspace>", ""},
	{[]string{http.MethodGet}, "user/workspaces/*/permission", "bb workspace permission get <workspace>", ""},
	{[]string{http.MethodGet}, "workspaces", "bb workspace list", ""},
	{[]string{http.MethodGet}, "workspaces/*", "bb workspace get <workspace>", ""},
	{[]string{http.MethodGet}, "user", "bb user me", ""},
	{[]string{http.MethodGet}, "user/emails", "bb user me --emails", ""},
	{[]string{http.MethodGet}, "users/*", "bb user get <user>", ""},

	// SSH and GPG keys
	{[]string{http.MethodGet}, "users/*/ssh-keys", "bb ssh-key list", ""},
	{[]string{http.MethodPost}, "users/*/ssh-keys", "bb ssh-key create", ""},
	{[]string{http.MethodGet}, "users/*/ssh-keys/*", "bb ssh-key get <fingerprint>", ""},
	{[]string{http.MethodDelete}, "users/*/ssh-keys/*", "bb ssh-key delete <fingerprint>", ""},
	{[]string{http.MethodGet}, "users/*/gpg-keys", "bb gpg-key list", ""},
	{[]string{http.MethodPost}, "users/*/gpg-keys", "bb gpg-key create", ""},
	{[]string{http.MethodGet}, "users/*/gpg-keys/*", "bb gpg-key get <fingerprint>", ""},
	{[]string{http.MethodDelete}, "users/*/gpg-keys/*", "bb gpg-key delete <fingerprint>", ""},
}

// checkNativeCommand refuses a request that a dedicated command already covers,
// unless --force was given. Writes are refused outright; reads only get a note
// on stderr, since reaching for the raw response of a GET is a fair way to see
// fields the formatted output leaves out.
func checkNativeCommand(method, endpoint string, bodyKeys []string) error {
	if apiOptions.Force {
		return nil
	}

	route := matchNativeRoute(method, endpoint)
	if route == nil {
		return nil
	}

	if method == http.MethodGet || method == http.MethodHead {
		fmt.Fprintf(os.Stderr, "Note: %q is already covered by \"%s\". Running the raw request anyway (--force silences this note).\n", method+" "+trimAPIRoot(endpoint), route.Command)
		return nil
	}

	if unexpressed := route.unexpressedFields(bodyKeys); len(unexpressed) > 0 {
		fmt.Fprintf(
			os.Stderr,
			"Note: %q is covered by \"%s\", but it cannot set %s. Running the raw request instead.\n",
			method+" "+trimAPIRoot(endpoint),
			route.Command,
			quoteAndJoin(unexpressed),
		)
		return nil
	}

	message := strings.Builder{}
	message.WriteString(fmt.Sprintf("%s %s is already covered by a dedicated command:\n\n", method, trimAPIRoot(endpoint)))
	message.WriteString(fmt.Sprintf("  %s\n\n", route.Command))
	if len(route.Adds) > 0 {
		message.WriteString(fmt.Sprintf("It %s, none of which a raw request does.\n", route.Adds))
	} else {
		message.WriteString("It validates the arguments and formats the response, which a raw request does not.\n")
	}
	if contract, found := route.contract(); found && len(contract.Caveat) > 0 {
		message.WriteString(contract.Caveat + "\n")
	}
	message.WriteString("Run \"bb api --force ...\" to send the request anyway.")

	return errors.New(message.String())
}

// quoteAndJoin renders body keys for a message: `"a"`, `"a" and "b"`, `"a", "b" and "c"`.
func quoteAndJoin(keys []string) string {
	quoted := make([]string, 0, len(keys))
	for _, key := range keys {
		quoted = append(quoted, fmt.Sprintf("%q", key))
	}
	switch len(quoted) {
	case 0:
		return ""
	case 1:
		return quoted[0]
	default:
		return strings.Join(quoted[:len(quoted)-1], ", ") + " and " + quoted[len(quoted)-1]
	}
}

// matchNativeRoute returns the first route covering the method and endpoint, or
// nil when no dedicated command exists for it.
func matchNativeRoute(method, endpoint string) *nativeRoute {
	segments := splitPath(trimAPIRoot(endpoint))
	if len(segments) == 0 {
		return nil
	}
	for i := range nativeRoutes {
		route := &nativeRoutes[i]
		if !matchesMethod(route, method) {
			continue
		}
		if matchesPattern(splitPath(route.Pattern), segments) {
			return route
		}
	}
	return nil
}

func matchesMethod(route *nativeRoute, method string) bool {
	if len(route.Methods) == 0 {
		return true
	}
	for _, candidate := range route.Methods {
		if candidate == method {
			return true
		}
	}
	return false
}

// matchesPattern compares pattern segments to path segments, where "*" matches
// exactly one segment and a trailing "**" matches one or more segments (for
// paths that embed a slash-bearing name, e.g. a tag or a download filename).
func matchesPattern(pattern, segments []string) bool {
	for i, want := range pattern {
		if want == "**" {
			return len(segments) > i
		}
		if i >= len(segments) {
			return false
		}
		if want != "*" && want != segments[i] {
			return false
		}
	}
	return len(segments) == len(pattern)
}

// trimAPIRoot reduces an endpoint to its path relative to the API version root,
// so a full URL, a "/2.0"-prefixed path and a bare path all compare the same.
// The query string is dropped.
func trimAPIRoot(endpoint string) string {
	path := endpoint
	if index := strings.IndexAny(path, "?#"); index >= 0 {
		path = path[:index]
	}
	if scheme := strings.Index(path, "://"); scheme >= 0 {
		path = path[scheme+len("://"):]
		if slash := strings.Index(path, "/"); slash >= 0 {
			path = path[slash:]
		} else {
			path = ""
		}
	}
	path = strings.Trim(path, "/")
	if path == "2.0" {
		return ""
	}
	return strings.TrimPrefix(path, "2.0/")
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if len(path) == 0 {
		return nil
	}
	return strings.Split(path, "/")
}
