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
	testutil.AssertEqual(t, item.Attributes["actor_login"], "actor", "actor_login")
	testutil.AssertEqual(t, item.Attributes["actor_avatar_url"], "https://avatar.url", "actor_avatar_url")
	testutil.AssertEqual(t, item.Attributes["repo_name"], "owner/repo", "repo_name")
	testutil.AssertEqual(t, item.Attributes["repo_url"], "https://api.github.com/repos/owner/repo", "repo_url")

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
	testutil.AssertEqual(t, item.Attributes["actor_login"], "", "actor_login")
	testutil.AssertEqual(t, item.Attributes["actor_avatar_url"], "", "actor_avatar_url")
	testutil.AssertEqual(t, item.Attributes["repo_name"], "", "repo_name")

	if item.Attributes["repo_url"] != "" {
		t.Errorf("expected empty repo_url, got %s", item.Attributes["repo_url"])
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

	if item.Attributes["actor_login"] != "" {
		t.Errorf("expected empty actor_login, got %s", item.Attributes["actor_login"])
	}

	if item.Attributes["repo_name"] != "" {
		t.Errorf("expected empty repo_name, got %s", item.Attributes["repo_name"])
	}
}
