package pullrequest

import (
	"github.com/delabrcd/bitbucket-cli/cmd/commit"
	"github.com/delabrcd/bitbucket-cli/cmd/repository"
)

type Endpoint struct {
	Branch     Branch                  `json:"branch"               mapstructure:"branch"`
	Commit     *commit.CommitReference `json:"commit,omitempty"     mapstructure:"commit"`
	Repository *repository.Repository  `json:"repository,omitempty" mapstructure:"repository"`
}
