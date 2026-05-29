package github

import (
	"testing"
	"time"

	gh "github.com/google/go-github/v69/github"
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
	mustNoError(t, err)
	assertExternalID(t, item, "12345")
	if item.ID.String() == "" {
		t.Error("expected non-empty ID")
	}
	if item.Source.Get() != "github" {
		t.Errorf("expected Source=github, got %s", item.Source.Get())
	}
	assertType(t, item, "PushEvent")
	assertEqual(t, item.ActorLogin.Get(), "actor", "ActorLogin")
	assertEqual(t, item.ActorAvatarURL, "https://avatar.url", "ActorAvatarURL")
	assertEqual(t, item.RepoName.Get(), "owner/repo", "RepoName")
	assertEqual(t, item.RepoURL, "https://api.github.com/repos/owner/repo", "RepoURL")
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
	mustNoError(t, err)
	assertExternalID(t, item, "999")
	if item.ID.String() == "" {
		t.Error("expected non-empty ID")
	}
	assertType(t, item, "WatchEvent")
	if item.ActorLogin.Get() != "" {
		t.Errorf("expected empty ActorLogin, got %s", item.ActorLogin.Get())
	}
	if item.ActorAvatarURL != "" {
		t.Errorf("expected empty ActorAvatarURL, got %s", item.ActorAvatarURL)
	}
	if item.RepoName.String() != "" {
		t.Errorf("expected empty RepoName, got %s", item.RepoName)
	}
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
	mustNoError(t, err)
	if item.ActorLogin.String() != "" {
		t.Errorf("expected empty ActorLogin, got %s", item.ActorLogin)
	}
	if item.RepoName.String() != "" {
		t.Errorf("expected empty RepoName, got %s", item.RepoName)
	}
}
