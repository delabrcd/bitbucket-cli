package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/itchyny/gojq"
)

// ApplyJQ evaluates the jq program expr against the JSON document in input and
// writes the results to w. A scalar string result is written raw (unquoted, like
// `jq -r`); every other result (object, array, number, bool, null) is written as
// indented JSON (two-space indent). Each result is written on its own line
// (newline-terminated). It returns an error if expr is not a valid jq program,
// if input is not valid JSON, or if evaluation produces a runtime/halt error.
func ApplyJQ(input []byte, expr string, w io.Writer) error {
	query, err := gojq.Parse(expr)
	if err != nil {
		return fmt.Errorf("failed to parse jq expression: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(input))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return fmt.Errorf("failed to decode JSON input: %w", err)
	}

	iter := query.Run(v)
	for {
		r, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := r.(error); ok {
			if haltErr, ok := err.(*gojq.HaltError); ok && haltErr.Value() == nil {
				break
			}
			return fmt.Errorf("jq evaluation error: %w", err)
		}

		if s, ok := r.(string); ok {
			if _, err := fmt.Fprintf(w, "%s\n", s); err != nil {
				return fmt.Errorf("failed to write jq result: %w", err)
			}
			continue
		}

		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal jq result: %w", err)
		}
		if _, err := fmt.Fprintf(w, "%s\n", b); err != nil {
			return fmt.Errorf("failed to write jq result: %w", err)
		}
	}

	return nil
}
