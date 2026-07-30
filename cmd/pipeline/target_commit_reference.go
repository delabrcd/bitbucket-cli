package pipeline

import (
	"encoding/json"

	"github.com/delabrcd/bitbucket-cli/cmd/commit"
	"github.com/delabrcd/bitbucket-cli/cmd/common"
	"github.com/gildas/go-errors"
)

// CommitReferenceTarget represents the target of a pipeline (branch, tag, etc.)
type CommitReferenceTarget struct {
	Selector *common.Selector        `json:"selector,omitempty" mapstructure:"selector"`
	Commit   *commit.CommitReference `json:"commit,omitempty"   mapstructure:"commit"`
}

func init() {
	targetRegistry.Add(CommitReferenceTarget{})
}

// GetType returns the target type
func (target CommitReferenceTarget) GetType() string {
	return "pipeline_commit_target"
}

// GetBranch returns an empty string, since this target names a commit
//
// implements Target
func (target CommitReferenceTarget) GetBranch() string {
	return ""
}

// GetDestination returns the target's destination
//
// implements Target
func (target CommitReferenceTarget) GetDestination() string {
	return ""
}

// GetPullRequestID returns 0, since this target is not a pullrequest
//
// implements Target
func (target CommitReferenceTarget) GetPullRequestID() uint64 {
	return 0
}

// GetCommit return the target's commit reference
//
// implements Target
func (target CommitReferenceTarget) GetCommit() *commit.CommitReference {
	return target.Commit
}

// MarshalJSON custom JSON marshalling for Target
//
// implements json.Marshaler
func (target CommitReferenceTarget) MarshalJSON() ([]byte, error) {
	type surrogate CommitReferenceTarget

	data, err := json.Marshal(struct {
		Type string `json:"type"`
		surrogate
	}{
		Type:      target.GetType(),
		surrogate: surrogate(target),
	})
	return data, errors.JSONMarshalError.Wrap(err)
}

// UnmarshalJSON custom JSON unmarshalling for Target
//
// implements json.Unmarshaler
func (target *CommitReferenceTarget) UnmarshalJSON(data []byte) error {
	type surrogate CommitReferenceTarget
	var inner struct {
		Type string `json:"type"`
		surrogate
	}

	if err := json.Unmarshal(data, &inner); err != nil {
		return errors.JSONUnmarshalError.WrapIfNotMe(err)
	}
	if inner.Type != target.GetType() {
		return errors.JSONUnmarshalError.Wrap(errors.InvalidType.With(inner.Type, target.GetType()))
	}
	*target = CommitReferenceTarget(inner.surrogate)

	return nil
}
