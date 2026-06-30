package pullrequest

import "testing"

func TestUnmetStatuses(t *testing.T) {
	statuses := []commitStatus{
		{Key: "pipeline", Name: "Pipeline", State: "SUCCESSFUL"},
		{Key: "codacy", Name: "Codacy", State: "FAILED"},
		{Key: "build", Name: "Build", State: "INPROGRESS"},
		{Key: "lint", Name: "Lint", State: "successful"}, // case-insensitive
		{Key: "deploy", Name: "Deploy", State: "STOPPED"},
	}

	unmet := unmetStatuses(statuses)
	if len(unmet) != 3 {
		t.Fatalf("expected 3 unmet statuses, got %d: %+v", len(unmet), unmet)
	}
	for _, status := range unmet {
		if status.State == "SUCCESSFUL" || status.State == "successful" {
			t.Errorf("successful status %q should not be reported as unmet", status.Key)
		}
	}
}

func TestUnmetStatusesAllSuccessful(t *testing.T) {
	statuses := []commitStatus{
		{Key: "a", State: "SUCCESSFUL"},
		{Key: "b", State: "SUCCESSFUL"},
	}
	if unmet := unmetStatuses(statuses); len(unmet) != 0 {
		t.Fatalf("expected no unmet statuses, got %d", len(unmet))
	}
}

func TestUnmetStatusesEmpty(t *testing.T) {
	if unmet := unmetStatuses(nil); len(unmet) != 0 {
		t.Fatalf("expected no unmet statuses for empty input, got %d", len(unmet))
	}
}

func TestCommitStatusLabel(t *testing.T) {
	if got := (commitStatus{Name: "Build", Key: "k"}).label(); got != "Build" {
		t.Errorf("expected Name as label, got %q", got)
	}
	if got := (commitStatus{Key: "only-key"}).label(); got != "only-key" {
		t.Errorf("expected Key fallback as label, got %q", got)
	}
}
