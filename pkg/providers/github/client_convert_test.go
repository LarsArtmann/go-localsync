package github

import (
	"testing"
	"time"

	gh "github.com/google/go-github/v69/github"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func TestConvertEvent_FullEvent(t *testing.T) {
	ghEvent := &gh.Event{
		ID:   new("12345"),
		Type: new("PushEvent"),
		Actor: &gh.User{
			Login:     new("actor"),
			AvatarURL: new("https://avatar.url"),
		},
		Repo: &gh.Repository{
			Name: new("owner/repo"),
			URL:  new("https://api.github.com/repos/owner/repo"),
		},
		CreatedAt: &gh.Timestamp{Time: time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)},
	}

	item, err := convertEvent(ghEvent)
	testutil.MustNoError(t, err)
	testutil.AssertEqual(t, item.ExternalID.Get(), "12345", "ExternalID")
	if item.ID.String() == "" {
		t.Error("expected non-empty ID")
	}
	if item.Source.Get() != "github" {
		t.Errorf("expected Source=github, got %s", item.Source.Get())
	}
	testutil.AssertEqual(t, item.Type.Get(), "PushEvent", "Type")
	testutil.AssertEqual(t, item.ActorLogin.Get(), "actor", "ActorLogin")
	testutil.AssertEqual(t, item.ActorAvatarURL, "https://avatar.url", "ActorAvatarURL")
	testutil.AssertEqual(t, item.RepoName.Get(), "owner/repo", "RepoName")
	testutil.AssertEqual(t, item.RepoURL, "https://api.github.com/repos/owner/repo", "RepoURL")
	if !item.CreatedAt.Equal(time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)) {
		t.Errorf("expected CreatedAt=2024-06-15 10:30:00, got %v", item.CreatedAt)
	}
	if len(item.RawJSON) == 0 {
		t.Error("expected non-empty RawJSON")
	}
}

func TestConvertEvent_MinimalEvent(t *testing.T) {
	ghEvent := &gh.Event{
		ID:        new("999"),
		Type:      new("WatchEvent"),
		CreatedAt: nil,
	}

	item, err := convertEvent(ghEvent)
	testutil.MustNoError(t, err)
	testutil.AssertEqual(t, item.ExternalID.Get(), "999", "ExternalID")
	if item.ID.String() == "" {
		t.Error("expected non-empty ID")
	}
	testutil.AssertEqual(t, item.Type.Get(), "WatchEvent", "Type")
	testutil.AssertEqual(t, item.ActorLogin.Get(), "", "ActorLogin")
	testutil.AssertEqual(t, item.ActorAvatarURL, "", "ActorAvatarURL")
	testutil.AssertEqual(t, item.RepoName.Get(), "", "RepoName")
	if item.RepoURL != "" {
		t.Errorf("expected empty RepoURL, got %s", item.RepoURL)
	}
	if item.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestConvertEvent_NilActorAndRepo(t *testing.T) {
	ghEvent := &gh.Event{
		ID:        new("1"),
		Type:      new("CreateEvent"),
		Actor:     nil,
		Repo:      nil,
		CreatedAt: &gh.Timestamp{Time: time.Now()},
	}

	item, err := convertEvent(ghEvent)
	testutil.MustNoError(t, err)
	if item.ActorLogin.String() != "" {
		t.Errorf("expected empty ActorLogin, got %s", item.ActorLogin)
	}
	if item.RepoName.String() != "" {
		t.Errorf("expected empty RepoName, got %s", item.RepoName)
	}
}
