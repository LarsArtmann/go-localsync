package model

// Canonical attribute keys (schema V3, ADR-0007). Providers fold their
// domain-specific content into Attributes using these well-known keys; the
// typed accessors below read them safely (missing keys read as "").
const (
	AttrActorLogin     = "actor_login"
	AttrActorAvatarURL = "actor_avatar_url"
	AttrRepoName       = "repo_name"
	AttrRepoURL        = "repo_url"
)

func (item Item) attr(key string) string {
	if item.Attributes == nil {
		return ""
	}

	return item.Attributes[key]
}

// ActorLogin returns the acting user's login (e.g. who pushed), if known.
func (item Item) ActorLogin() string { return item.attr(AttrActorLogin) }

// ActorAvatarURL returns the acting user's avatar URL, if known.
func (item Item) ActorAvatarURL() string { return item.attr(AttrActorAvatarURL) }

// RepoName returns the repository in owner/name form, if known.
func (item Item) RepoName() string { return item.attr(AttrRepoName) }

// RepoURL returns the repository's web URL, if known.
func (item Item) RepoURL() string { return item.attr(AttrRepoURL) }
