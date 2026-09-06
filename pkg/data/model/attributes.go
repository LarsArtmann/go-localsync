package model

// Canonical attribute keys (schema V3, ADR-0007). Providers fold their
// domain-specific content into Attributes using these well-known keys; the
// typed accessors below read them safely (missing keys read as "") and the
// With* helpers write them without raw-key typos.
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

// setAttr returns item with key set to value, lazily allocating the Attributes
// map. The map is copied before mutation so Items sharing a backing map (the
// common case when items round-trip through events) never see each other's
// writes. Write-helpers mirror the typed readers.
func (item Item) setAttr(key, value string) Item {
	cloned := make(map[string]string, len(item.Attributes)+1)
	for k, v := range item.Attributes {
		cloned[k] = v
	}

	cloned[key] = value
	item.Attributes = cloned

	return item
}

// ActorLogin returns the acting user's login (e.g. who pushed), if known.
func (item Item) ActorLogin() string { return item.attr(AttrActorLogin) }

// WithActorLogin returns a copy of item with the acting user's login set.
func (item Item) WithActorLogin(login string) Item { return item.setAttr(AttrActorLogin, login) }

// ActorAvatarURL returns the acting user's avatar URL, if known.
func (item Item) ActorAvatarURL() string { return item.attr(AttrActorAvatarURL) }

// WithActorAvatarURL returns a copy of item with the actor's avatar URL set.
func (item Item) WithActorAvatarURL(url string) Item { return item.setAttr(AttrActorAvatarURL, url) }

// RepoName returns the repository in owner/name form, if known.
func (item Item) RepoName() string { return item.attr(AttrRepoName) }

// WithRepoName returns a copy of item with the repository name set.
func (item Item) WithRepoName(name string) Item { return item.setAttr(AttrRepoName, name) }

// RepoURL returns the repository's web URL, if known.
func (item Item) RepoURL() string { return item.attr(AttrRepoURL) }

// WithRepoURL returns a copy of item with the repository URL set.
func (item Item) WithRepoURL(url string) Item { return item.setAttr(AttrRepoURL, url) }
