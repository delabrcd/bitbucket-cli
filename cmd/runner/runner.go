package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gildas/bitbucket-cli/cmd/common"
	"github.com/gildas/bitbucket-cli/cmd/profile"
	"github.com/gildas/bitbucket-cli/cmd/repository"
	"github.com/gildas/bitbucket-cli/cmd/workspace"
	"github.com/gildas/go-core"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

// Runner represents a Bitbucket Pipelines self-hosted runner.
//
// Runners live at two scopes: repository (default) and workspace
// (--workspace-level). The two scopes share the same object shape and only
// differ by the API base path (see runnerBasePath).
type Runner struct {
	Type        string       `json:"type"                   mapstructure:"type"`
	UUID        common.UUID  `json:"uuid"                   mapstructure:"uuid"`
	Name        string       `json:"name"                   mapstructure:"name"`
	Labels      []string     `json:"labels"                 mapstructure:"labels"`
	State       RunnerState  `json:"state"                  mapstructure:"state"`
	OAuthClient *OAuthClient `json:"oauth_client,omitempty" mapstructure:"oauth_client"`
	CreatedOn   time.Time    `json:"created_on"             mapstructure:"created_on"`
	UpdatedOn   time.Time    `json:"updated_on"             mapstructure:"updated_on"`
}

// RunnerState is the runtime state of a runner.
type RunnerState struct {
	Status    string        `json:"status"     mapstructure:"status"`
	Version   RunnerVersion `json:"version"    mapstructure:"version"`
	Cordoned  bool          `json:"cordoned"   mapstructure:"cordoned"`
	UpdatedOn time.Time     `json:"updated_on" mapstructure:"updated_on"`
}

// RunnerVersion holds the runner agent version reported by Bitbucket.
//
// Bitbucket reports the running agent version in the "current" field.
type RunnerVersion struct {
	Current string `json:"current" mapstructure:"current"`
}

// OAuthClient holds the credentials Bitbucket mints for a newly created runner.
//
// The Secret is only ever returned once, by the create call.
type OAuthClient struct {
	ID            string `json:"id"               mapstructure:"id"`
	Secret        string `json:"secret,omitempty" mapstructure:"secret"`
	TokenEndpoint string `json:"token_endpoint" mapstructure:"token_endpoint"`
	Audience      string `json:"audience"       mapstructure:"audience"`
}

// Command represents this folder's command
var Command = &cobra.Command{
	Use:   "runner",
	Short: "Manage Pipelines self-hosted runners (repository or workspace scope)",
	Long: `Manage Bitbucket Pipelines self-hosted runners.

By default commands operate on the runners of the current repository. Pass
--workspace-level to operate on the workspace's shared runners instead.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Runner requires a subcommand:")
		for _, command := range cmd.Commands() {
			fmt.Println(command.Name())
		}
	},
}

// Options shared by every runner subcommand, set by the persistent flag below.
var Options struct {
	WorkspaceLevel bool
}

var columns = common.Columns[Runner]{
	{Name: "name", DefaultSorter: true, Compare: func(a, b Runner) bool {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) == -1
	}},
	{Name: "uuid", DefaultSorter: false, Compare: func(a, b Runner) bool {
		return strings.Compare(strings.ToLower(a.UUID.String()), strings.ToLower(b.UUID.String())) == -1
	}},
	{Name: "status", DefaultSorter: false, Compare: func(a, b Runner) bool {
		return strings.Compare(strings.ToLower(a.State.Status), strings.ToLower(b.State.Status)) == -1
	}},
	{Name: "labels", DefaultSorter: false, Compare: func(a, b Runner) bool {
		return strings.Compare(strings.ToLower(strings.Join(a.Labels, ",")), strings.ToLower(strings.Join(b.Labels, ","))) == -1
	}},
	{Name: "version", DefaultSorter: false, Compare: func(a, b Runner) bool {
		return strings.Compare(a.State.Version.Current, b.State.Version.Current) == -1
	}},
	{Name: "cordoned", DefaultSorter: false, Compare: func(a, b Runner) bool {
		return !a.State.Cordoned && b.State.Cordoned
	}},
	{Name: "created", DefaultSorter: false, Compare: func(a, b Runner) bool {
		return a.CreatedOn.Before(b.CreatedOn)
	}},
}

func init() {
	Command.PersistentFlags().BoolVarP(&Options.WorkspaceLevel, "workspace-level", "W", false, "Operate on the workspace's shared runners instead of the current repository's runners")
}

// GetHeaders gets the header for a table
//
// implements common.Tableable
func (runner Runner) GetHeaders(cmd *cobra.Command) []string {
	if cmd != nil && cmd.Flag("columns") != nil && cmd.Flag("columns").Changed {
		if columns, err := cmd.Flags().GetStringSlice("columns"); err == nil {
			return core.Map(columns, func(column string) string { return strings.ReplaceAll(column, "_", " ") })
		}
	}
	return []string{"UUID", "Name", "Status", "Labels", "Version"}
}

// GetRow gets the row for a table
//
// implements common.Tableable
func (runner Runner) GetRow(headers []string) []string {
	var row []string

	for _, header := range headers {
		switch strings.ToLower(header) {
		case "uuid", "id":
			row = append(row, runner.UUID.String())
		case "name":
			row = append(row, runner.Name)
		case "status", "state":
			row = append(row, runner.State.Status)
		case "labels":
			row = append(row, strings.Join(runner.Labels, ", "))
		case "version":
			row = append(row, runner.State.Version.Current)
		case "cordoned":
			row = append(row, fmt.Sprintf("%t", runner.State.Cordoned))
		case "created", "created_on":
			row = append(row, runner.CreatedOn.Format(time.RFC3339))
		case "updated", "updated_on":
			row = append(row, runner.UpdatedOn.Format(time.RFC3339))
		}
	}
	return row
}

// String gets a string representation
//
// implements fmt.Stringer
func (runner Runner) String() string {
	return runner.Name
}

// runnerBasePath builds the API path for the current scope (repository by
// default, workspace when --workspace-level is set), appending paths.
//
// Repository: /repositories/{workspace}/{repo}/pipelines-config/runners[/...]
// Workspace:  /workspaces/{workspace}/pipelines-config/runners[/...]
func runnerBasePath(ctx context.Context, cmd *cobra.Command, paths ...string) (string, error) {
	segments := append([]string{"pipelines-config", "runners"}, paths...)

	if Options.WorkspaceLevel {
		ws, err := workspace.GetWorkspace(ctx, cmd)
		if err != nil {
			return "", err
		}
		return "/" + strings.Join(append([]string{"workspaces", ws.Slug}, segments...), "/"), nil
	}

	repo, err := repository.GetRepository(ctx, cmd)
	if err != nil {
		return "", err
	}
	return repo.GetPath(segments...), nil
}

// NormalizeRunnerUUID wraps a bare runner UUID in the braces Bitbucket expects
// in the path, leaving an already-braced value untouched.
func NormalizeRunnerUUID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 0 || strings.HasPrefix(value, "{") {
		return value
	}
	return "{" + value + "}"
}

// GetRunnerUUIDs lists the runner UUIDs of the current scope, for completion.
func GetRunnerUUIDs(ctx context.Context, cmd *cobra.Command) ([]string, error) {
	log := logger.Must(logger.FromContext(ctx)).Child("runner", "getuuids")

	path, err := runnerBasePath(ctx, cmd)
	if err != nil {
		return []string{}, err
	}
	runners, err := profile.GetAll[Runner](ctx, cmd, path)
	if err != nil {
		log.Errorf("Failed to get runners", err)
		return []string{}, err
	}
	uuids := make([]string, 0, len(runners))
	for _, runner := range runners {
		uuids = append(uuids, runner.UUID.String())
	}
	return uuids, nil
}
