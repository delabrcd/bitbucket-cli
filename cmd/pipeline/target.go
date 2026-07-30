package pipeline

import (
	"strings"

	"github.com/delabrcd/bitbucket-cli/cmd/commit"
	"github.com/gildas/go-core"
	"github.com/gildas/go-errors"
)

// Target represents the target of a pipeline (branch, tag, etc.)
type Target interface {
	core.TypeCarrier
	// GetBranch returns the branch being built, which for a pullrequest target is
	// its source. GetDestination returns the merge destination instead.
	GetBranch() string
	GetDestination() string
	GetPullRequestID() uint64
	GetCommit() *commit.CommitReference
}

var targetRegistry = core.TypeRegistry{}

// UnmarshalTarget unmarshals a Target from JSON data
func UnmarshalTarget(payload []byte) (Target, error) {
	target, err := targetRegistry.UnmarshalJSON(payload)
	if err != nil {
		if strings.HasPrefix(err.Error(), "Missing JSON Property") {
			return nil, errors.JSONUnmarshalError.Wrap(errors.ArgumentMissing.With("type"))
		}
		if strings.HasPrefix(err.Error(), "Unsupported Type") {
			keys := make([]string, 0, len(targetRegistry))
			for key := range targetRegistry {
				keys = append(keys, key)
			}
			return nil, errors.JSONUnmarshalError.Wrap(errors.InvalidType.With(strings.TrimSuffix(strings.TrimPrefix(err.Error(), `Unsupported Type "`), `"`), strings.Join(keys, ", ")))
		}
		return nil, err
	}
	return target.(Target), nil
}
