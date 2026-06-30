package runner_test

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/delabrcd/bitbucket-cli/cmd/runner"
	"github.com/gildas/go-logger"
	"github.com/stretchr/testify/suite"
)

type RunnerSuite struct {
	suite.Suite
	Name   string
	Logger *logger.Logger
	Start  time.Time
}

func TestRunnerSuite(t *testing.T) {
	suite.Run(t, new(RunnerSuite))
}

// *****************************************************************************
// Suite Tools

func (suite *RunnerSuite) SetupSuite() {
	suite.Name = strings.TrimSuffix(reflect.TypeOf(suite).Elem().Name(), "Suite")
	suite.Logger = logger.Create("test",
		&logger.FileStream{
			Path:         fmt.Sprintf("./log/test-%s.log", strings.ToLower(suite.Name)),
			Unbuffered:   true,
			SourceInfo:   true,
			FilterLevels: logger.NewLevelSet(logger.TRACE),
		},
	).Child("test", "test")
	suite.Logger.Infof("Suite Start: %s %s", suite.Name, strings.Repeat("=", 80-14-len(suite.Name)))
}

func (suite *RunnerSuite) TearDownSuite() {
	suite.Logger.Infof("Suite End: %s %s", suite.Name, strings.Repeat("=", 80-12-len(suite.Name)))
}

func (suite *RunnerSuite) LoadTestData(filename string) []byte {
	data, err := os.ReadFile(fmt.Sprintf("../../testdata/%s", filename))
	if err != nil {
		suite.T().Fatal(err)
	}
	return data
}

func (suite *RunnerSuite) UnmarshalData(filename string, v any) error {
	data := suite.LoadTestData(filename)
	suite.Logger.Infof("Loaded %s: %s", filename, string(data))
	return json.Unmarshal(data, v)
}

// *****************************************************************************

func (suite *RunnerSuite) TestCanUnmarshal() {
	var r runner.Runner
	err := suite.UnmarshalData("runner.json", &r)
	suite.Require().NoError(err)
	suite.Assert().Equal("pipeline_runner", r.Type)
	suite.Assert().Equal("{670ea7af-1234-4f6b-9bd8-a8aef6c1f5e6}", r.UUID.String())
	suite.Assert().Equal("linux-builder-01", r.Name)
	suite.Assert().Equal([]string{"self.hosted", "linux"}, r.Labels)
	suite.Assert().Equal("ONLINE", r.State.Status)
	suite.Assert().Equal("5.9.0", r.State.Version.Current)
	suite.Assert().False(r.State.Cordoned)
	suite.Require().NotNil(r.OAuthClient)
	suite.Assert().Equal("abc123", r.OAuthClient.ID)
	suite.Assert().Equal("sshhh-secret", r.OAuthClient.Secret)
}

func (suite *RunnerSuite) TestGetRow() {
	var r runner.Runner
	suite.Require().NoError(suite.UnmarshalData("runner.json", &r))

	headers := r.GetHeaders(nil)
	suite.Assert().Equal([]string{"UUID", "Name", "Status", "Labels", "Version"}, headers)

	row := r.GetRow(headers)
	suite.Require().Len(row, len(headers))
	suite.Assert().Equal("{670ea7af-1234-4f6b-9bd8-a8aef6c1f5e6}", row[0])
	suite.Assert().Equal("linux-builder-01", row[1])
	suite.Assert().Equal("ONLINE", row[2])
	suite.Assert().Equal("self.hosted, linux", row[3])
	suite.Assert().Equal("5.9.0", row[4])
}

func (suite *RunnerSuite) TestRunnerString() {
	var r runner.Runner
	suite.Require().NoError(suite.UnmarshalData("runner.json", &r))
	suite.Assert().Equal("linux-builder-01", r.String())
}

func (suite *RunnerSuite) TestRunnersTableable() {
	var r runner.Runner
	suite.Require().NoError(suite.UnmarshalData("runner.json", &r))
	runners := runner.Runners{r}
	suite.Assert().Equal(1, runners.Size())
	headers := runners.GetHeaders(nil)
	suite.Assert().Equal(r.GetRow(headers), runners.GetRowAt(0, headers))
	suite.Assert().Empty(runners.GetRowAt(5, headers))
}

func (suite *RunnerSuite) TestNormalizeRunnerUUID() {
	braced := "{670ea7af-1234-4f6b-9bd8-a8aef6c1f5e6}"
	bare := "670ea7af-1234-4f6b-9bd8-a8aef6c1f5e6"
	suite.Assert().Equal(braced, runner.NormalizeRunnerUUID(bare))
	suite.Assert().Equal(braced, runner.NormalizeRunnerUUID(braced))
	suite.Assert().Equal(braced, runner.NormalizeRunnerUUID("  "+bare+"  "))
	suite.Assert().Equal("", runner.NormalizeRunnerUUID(""))
}
