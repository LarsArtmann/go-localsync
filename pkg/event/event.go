package event

import (
	"encoding/json"
	"time"
)

type Event struct {
	GithubID       string          `json:"githubId"`
	Type           string          `json:"type"`
	ActorLogin     string          `json:"actorLogin,omitempty"`
	ActorAvatarURL string          `json:"actorAvatarUrl,omitempty"`
	RepoName       string          `json:"repoName,omitempty"`
	RepoURL        string          `json:"repoUrl,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	RawJSON        json.RawMessage `json:"rawJson"`
}
