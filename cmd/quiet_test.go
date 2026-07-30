package cmd

import "testing"

// Profile.Print reads the "quiet" flag, so --silent has to be visible through it.
func TestSilentIsASynonymForQuiet(t *testing.T) {
	flags := RootCmd.PersistentFlags()

	defer func() {
		CmdOptions.Quiet = false
		_ = flags.Set("quiet", "false")
		_ = flags.Set("silent", "false")
	}()

	if quiet, err := flags.GetBool("quiet"); err != nil || quiet {
		t.Fatalf("quiet defaulted to %v (err %v), want false", quiet, err)
	}

	if err := flags.Set("silent", "true"); err != nil {
		t.Fatalf("setting --silent: %v", err)
	}
	if !CmdOptions.Quiet {
		t.Error("--silent did not set CmdOptions.Quiet")
	}
	quiet, err := flags.GetBool("quiet")
	if err != nil {
		t.Fatalf("reading --quiet: %v", err)
	}
	if !quiet {
		t.Error("--silent is not visible through the quiet flag, so Profile.Print would ignore it")
	}
}

// -q stays bound to --jq, matching gh.
func TestShortQIsJQ(t *testing.T) {
	flag := RootCmd.PersistentFlags().ShorthandLookup("q")
	if flag == nil {
		t.Fatal("-q is not registered")
	}
	if flag.Name != "jq" {
		t.Errorf("-q is bound to --%s, want --jq", flag.Name)
	}
	if RootCmd.PersistentFlags().ShorthandLookup("Q") != nil {
		t.Error("-Q is registered; --quiet is meant to have no short form")
	}
}
