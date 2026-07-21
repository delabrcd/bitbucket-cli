package profile_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/delabrcd/bitbucket-cli/cmd/profile"
	"github.com/spf13/cobra"
)

type testItem struct {
	ID string `json:"id"`
}

func (suite *ProfileSuite) TestGetAll_OriginalQueryIsPreservedForNextMissingParams() {
	oldCurrent := profile.Current
	defer func() { profile.Current = oldCurrent }()

	const filter = `target.ref_name="my-branch"`
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if r.URL.Path == "/pipelines" {
			if r.URL.Query().Get("page") == "" {
				suite.Assert().Equal(filter, q, "initial request should include original q")
				resp := map[string]interface{}{
					"values": []map[string]string{{"id": "1"}},
					"next":   server.URL + "/pipelines?page=2&pagelen=1",
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			suite.Assert().Equal(filter, q, "second request should include original q even when next omits it")
			resp := map[string]interface{}{
				"values": []map[string]string{{"id": "2"}},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	apiRoot, err := url.Parse(server.URL)
	suite.Require().NoError(err)
	profile.Current = &profile.Profile{APIRoot: apiRoot, DefaultPageLength: 0, AccessToken: "dummy-token"}

	cmd := &cobra.Command{}
	cmd.Flags().String("profile", "", "")
	cmd.Flags().Int("page-length", 0, "")
	cmd.SetContext(suite.Context)
	items, err := profile.GetAll[testItem](suite.Context, cmd, server.URL+"/pipelines?pagelen=1&q="+url.QueryEscape(filter))
	suite.Require().NoError(err)
	suite.Require().Len(items, 2)
	suite.Require().Equal("1", items[0].ID)
	suite.Require().Equal("2", items[1].ID)
}

func (suite *ProfileSuite) TestAuthorize_DoesNotPanicWhenNoTokenAvailable() {
	oldCurrent := profile.Current
	defer func() { profile.Current = oldCurrent }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"values": []map[string]string{}})
	}))
	defer server.Close()

	apiRoot, err := url.Parse(server.URL)
	suite.Require().NoError(err)
	// A profile that has no user (so send() does not use basic auth), no access
	// token, no cached token and no client secret. loadAccessToken leaves
	// profile.token nil while returning a nil error, so authorize must not
	// dereference the nil token.
	profile.Current = &profile.Profile{Name: "no-token", APIRoot: apiRoot, ClientID: "clientid-without-secret"}

	cmd := &cobra.Command{}
	cmd.Flags().String("profile", "", "")
	cmd.SetContext(suite.Context)
	suite.Require().NotPanics(func() {
		_ = profile.Current.Get(suite.Context, cmd, "/workspaces", &testItem{})
	}, "authorize must not panic when no token is available")
}

func (suite *ProfileSuite) TestGetAll_DoesNotOverwriteExistingNextParams() {
	oldCurrent := profile.Current
	defer func() { profile.Current = oldCurrent }()

	const originalFilter = `target.ref_name="original"`
	const nextFilter = `target.ref_name="different"`
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if r.URL.Path == "/pipelines" {
			if r.URL.Query().Get("page") == "" {
				suite.Assert().Equal(originalFilter, q, "initial request should include original q")
				resp := map[string]interface{}{
					"values": []map[string]string{{"id": "1"}},
					"next":   server.URL + "/pipelines?page=2&pagelen=1&q=" + url.QueryEscape(nextFilter),
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			suite.Assert().Equal(nextFilter, q, "existing q on next URL must not be overwritten")
			resp := map[string]interface{}{
				"values": []map[string]string{{"id": "2"}},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	apiRoot, err := url.Parse(server.URL)
	suite.Require().NoError(err)
	profile.Current = &profile.Profile{APIRoot: apiRoot, DefaultPageLength: 0, AccessToken: "dummy-token"}

	cmd := &cobra.Command{}
	cmd.Flags().String("profile", "", "")
	cmd.Flags().Int("page-length", 0, "")
	cmd.SetContext(suite.Context)
	items, err := profile.GetAll[testItem](suite.Context, cmd, server.URL+"/pipelines?pagelen=1&q="+url.QueryEscape(originalFilter))
	suite.Require().NoError(err)
	suite.Require().Len(items, 2)
	suite.Require().Equal("1", items[0].ID)
	suite.Require().Equal("2", items[1].ID)
}
