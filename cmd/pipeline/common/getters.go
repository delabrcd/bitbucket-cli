package plcommon

import (
	"context"
	"fmt"
	"strings"

	"github.com/delabrcd/bitbucket-cli/cmd/profile"
	"github.com/delabrcd/bitbucket-cli/cmd/repository"
	"github.com/gildas/go-core"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

type PipelineID struct {
	ID int `json:"build_number" mapstructure:"build_number"`
}

// GetPipelineIDs gets the IDs of the pipelines
func GetPipelineIDs(context context.Context, cmd *cobra.Command, args []string, toComplete string) (ids []string, err error) {
	log := logger.Must(logger.FromContext(context)).Child(nil, "getpipelines")

	repository, err := repository.GetRepository(cmd.Context(), cmd)
	if err != nil {
		return []string{}, err
	}

	log.Infof("Getting pipelines")
	pipelines, err := profile.GetAll[PipelineID](
		log.ToContext(context),
		cmd,
		repository.GetPath("pipelines"),
	)
	if err != nil {
		log.Errorf("Failed to get pipelines", err)
		return []string{}, err
	}

	ids = core.Map(pipelines, func(pipeline PipelineID) string { return fmt.Sprintf("%d", pipeline.ID) })
	core.Sort(ids, func(a, b string) bool { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) == -1 })
	return ids, nil
}

// GetLatestPipelineID returns the build number of the most recently created
// pipeline of the current repository.
func GetLatestPipelineID(context context.Context, cmd *cobra.Command) (id string, err error) {
	log := logger.Must(logger.FromContext(context)).Child(nil, "getlatestpipeline")

	prof, err := profile.GetProfileFromCommand(context, cmd)
	if err != nil {
		return "", err
	}
	repository, err := repository.GetRepository(cmd.Context(), cmd)
	if err != nil {
		return "", err
	}

	var page profile.PaginatedResources[PipelineID]
	if err = prof.Get(
		log.ToContext(context),
		cmd,
		repository.GetPath("pipelines")+"?sort=-created_on&pagelen=1",
		&page,
	); err != nil {
		return "", err
	}
	if len(page.Values) == 0 {
		return "", errors.NotFound.With("pipeline", "latest")
	}
	return fmt.Sprintf("%d", page.Values[0].ID), nil
}
