package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/delabrcd/bitbucket-cli/cmd"
	"github.com/gildas/go-logger"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	if len(os.Getenv("LOG_DESTINATION")) == 0 {
		os.Setenv("LOG_DESTINATION", "nil")
	}
	log := logger.Create(APP)
	defer log.Flush()
	// Identify by the name the binary was invoked as (e.g. "bb" for the
	// release, "bb-local" for a dev build), falling back to APP.
	cmd.RootCmd.Use = invokedName()
	cmd.RootCmd.Version = Version()
	err := cmd.Execute(log.ToContext(context.Background()))
	if err != nil {
		log.Fatalf("Failed to execute command", err)
		os.Exit(1)
	}
}

// invokedName returns the base name the binary was launched as, so the help
// and usage output reflect the actual command (e.g. "bb-local" for a dev
// build symlinked onto PATH). It falls back to APP when the name is empty or
// looks like a temporary "go run" build artifact.
func invokedName() string {
	name := filepath.Base(os.Args[0])
	if name == "" || name == "." || name == string(filepath.Separator) {
		return APP
	}
	return name
}
