package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/delabrcd/bitbucket-cli/cmd/common"
	"github.com/delabrcd/bitbucket-cli/cmd/profile"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
	"github.com/gildas/go-request"
	"github.com/spf13/cobra"
)

// Command represents this folder's command
var Command = &cobra.Command{
	Use:   "api <endpoint>",
	Short: "Make an authenticated request to the BitBucket Cloud REST API",
	Long: `Make an authenticated HTTP request to the BitBucket Cloud REST API and print the response.

The endpoint argument is a path (e.g. "repositories/{workspace}/{repo}") that is
joined to the API root and the "2.0" version prefix. A leading "/" is optional. A
full URL (https://...) is used verbatim, which is handy for following pagination
"next" links by hand.

The method defaults to GET, or to POST when any field or request body is provided.

Examples:
  # Get the current user
  bb api user

  # List a repository's pull requests
  bb api repositories/inficon-global/cdt-haps/pullrequests

  # Create a pull request with typed fields (numbers, booleans, nested via JSON body)
  bb api -X POST repositories/{workspace}/{repo}/pullrequests \
    -f title="My PR" -f source.branch.name=feature/x

  # Send a raw JSON body from a file (or "-" for stdin)
  bb api -X POST repositories/{workspace}/{repo}/pullrequests --input body.json

  # Follow pagination and merge every page's values into one result
  bb api --paginate repositories/{workspace}/{repo}/pullrequests`,
	Args: cobra.ExactArgs(1),
	RunE: apiProcess,
}

var apiOptions struct {
	Method      string
	RawFields   []string
	Fields      []string
	Headers     []string
	Input       string
	ContentType string
	Paginate    bool
	Include     bool
	JQ          string
}

func init() {
	Command.Flags().StringVarP(&apiOptions.Method, "method", "X", "", "HTTP method (default GET, or POST when a body/field is given)")
	Command.Flags().StringArrayVarP(&apiOptions.RawFields, "raw-field", "f", nil, "Add a string body parameter in key=value format (repeatable)")
	Command.Flags().StringArrayVarP(&apiOptions.Fields, "field", "F", nil, "Add a typed body parameter in key=value format; values true/false/null/numbers are typed, @file reads a file, @- reads stdin (repeatable)")
	Command.Flags().StringArrayVarP(&apiOptions.Headers, "header", "H", nil, "Add an HTTP request header in key:value format (repeatable)")
	Command.Flags().StringVar(&apiOptions.Input, "input", "", "File to read the request body from (\"-\" for stdin)")
	Command.Flags().StringVar(&apiOptions.ContentType, "content-type", "", "Content-Type of the request body (default application/json)")
	Command.Flags().BoolVar(&apiOptions.Paginate, "paginate", false, "Follow \"next\" links, merging every page's values into a single result")
	Command.Flags().BoolVarP(&apiOptions.Include, "include", "i", false, "Include the response status line and headers in the output")

	Command.MarkFlagsMutuallyExclusive("input", "field")
	Command.MarkFlagsMutuallyExclusive("input", "raw-field")
	_ = Command.MarkFlagFilename("input")
}

func apiProcess(cmd *cobra.Command, args []string) (err error) {
	log := logger.Must(logger.FromContext(cmd.Context())).Child("api", "api")

	currentProfile, err := profile.GetProfileFromCommand(cmd.Context(), cmd)
	if err != nil {
		return err
	}

	if jq := cmd.Flag("jq"); jq != nil && jq.Changed {
		apiOptions.JQ = jq.Value.String()
	}

	body, payloadType, err := buildBody()
	if err != nil {
		return err
	}

	method := strings.ToUpper(apiOptions.Method)
	if method == "" {
		if body != nil {
			method = http.MethodPost
		} else {
			method = http.MethodGet
		}
	}

	headers, err := parseHeaders(apiOptions.Headers)
	if err != nil {
		return err
	}

	endpoint := normalizeEndpoint(args[0])
	log.Infof("Calling %s %s", method, endpoint)

	if apiOptions.Paginate {
		return paginate(cmd, currentProfile, method, endpoint, payloadType, headers, body)
	}

	result, err := currentProfile.CallAPI(cmd.Context(), cmd, method, endpoint, payloadType, headers, body)
	return output(result, err)
}

// buildBody assembles the request body from --field/--raw-field or --input.
// It returns the payload (nil when none was given) and its content type.
func buildBody() (body interface{}, payloadType string, err error) {
	payloadType = apiOptions.ContentType

	if len(apiOptions.Input) > 0 {
		var data []byte
		if apiOptions.Input == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(apiOptions.Input)
		}
		if err != nil {
			return nil, "", errors.RuntimeError.Wrap(err)
		}
		if payloadType == "" {
			payloadType = "application/json"
		}
		// Wrap in a reader so go-request sends the bytes verbatim instead of re-encoding them.
		return bytes.NewReader(data), payloadType, nil
	}

	if len(apiOptions.Fields) == 0 && len(apiOptions.RawFields) == 0 {
		return nil, payloadType, nil
	}

	fields := map[string]interface{}{}
	for _, raw := range apiOptions.RawFields {
		key, value, found := strings.Cut(raw, "=")
		if !found {
			return nil, "", errors.ArgumentInvalid.With("raw-field", raw)
		}
		fields[key] = value
	}
	for _, raw := range apiOptions.Fields {
		key, value, found := strings.Cut(raw, "=")
		if !found {
			return nil, "", errors.ArgumentInvalid.With("field", raw)
		}
		typed, terr := typedFieldValue(value)
		if terr != nil {
			return nil, "", terr
		}
		fields[key] = typed
	}
	if payloadType == "" {
		payloadType = "application/json"
	}
	return fields, payloadType, nil
}

// typedFieldValue converts a --field value to a typed value, mirroring gh api:
// true/false/null and numbers are typed, @file reads a file ("@-" reads stdin),
// everything else stays a string.
func typedFieldValue(value string) (interface{}, error) {
	switch {
	case value == "true":
		return true, nil
	case value == "false":
		return false, nil
	case value == "null":
		return nil, nil
	case strings.HasPrefix(value, "@"):
		var (
			data []byte
			err  error
		)
		if value == "@-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(value[1:])
		}
		if err != nil {
			return nil, errors.RuntimeError.Wrap(err)
		}
		return string(data), nil
	}
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i, nil
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f, nil
	}
	return value, nil
}

// normalizeEndpoint passes full URLs through untouched (e.g. pagination "next"
// links) and gives every relative path a leading "/" so it is joined to the API
// root, letting callers write "user" as well as "/user".
func normalizeEndpoint(endpoint string) string {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	if !strings.HasPrefix(endpoint, "/") {
		return "/" + endpoint
	}
	return endpoint
}

func parseHeaders(raw []string) (map[string]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	headers := map[string]string{}
	for _, header := range raw {
		key, value, found := strings.Cut(header, ":")
		if !found {
			return nil, errors.ArgumentInvalid.With("header", header)
		}
		headers[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return headers, nil
}

// output writes the response body to stdout (and headers with --include), then
// returns the request error so the process exit code reflects HTTP failures.
func output(result *request.Content, err error) error {
	if result != nil {
		if apiOptions.Include {
			if result.StatusCode > 0 {
				fmt.Printf("HTTP %d %s\n", result.StatusCode, http.StatusText(result.StatusCode))
			}
			_ = result.Headers.Write(os.Stdout)
			fmt.Println()
		}
		if len(result.Data) > 0 {
			if len(apiOptions.JQ) > 0 {
				if jqErr := common.ApplyJQ(result.Data, apiOptions.JQ, os.Stdout); jqErr != nil {
					return jqErr
				}
			} else {
				os.Stdout.Write(result.Data)
				if !bytes.HasSuffix(result.Data, []byte("\n")) {
					fmt.Println()
				}
			}
		}
	}
	return err
}

// page is the subset of a BitBucket paginated response we need to walk pages.
type page struct {
	Values []json.RawMessage `json:"values"`
	Next   string            `json:"next"`
}

// paginate follows "next" links, merging every page's values into a single
// {"values": [...]} document printed once at the end.
func paginate(cmd *cobra.Command, currentProfile *profile.Profile, method, endpoint, payloadType string, headers map[string]string, body interface{}) error {
	values := []json.RawMessage{}
	uripath := endpoint
	for {
		result, err := currentProfile.CallAPI(cmd.Context(), cmd, method, uripath, payloadType, headers, body)
		if err != nil {
			return output(result, err)
		}
		var current page
		if jerr := json.Unmarshal(result.Data, &current); jerr != nil {
			// Not a paginated payload, just emit it as-is.
			return output(result, nil)
		}
		values = append(values, current.Values...)
		if len(current.Next) == 0 {
			break
		}
		uripath = current.Next
	}
	merged, err := json.MarshalIndent(map[string]interface{}{"values": values}, "", "  ")
	if err != nil {
		return errors.JSONMarshalError.Wrap(err)
	}
	if len(apiOptions.JQ) > 0 {
		return common.ApplyJQ(merged, apiOptions.JQ, os.Stdout)
	}
	fmt.Println(string(merged))
	return nil
}
