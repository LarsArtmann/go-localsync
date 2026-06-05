package cqrs

import (
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

func testProviderItem() *provider.Item {
	return &provider.Item{
		ID:             id.NewItemID(),
		ExternalID:     id.NewExternalID("12345"),
		Source:         id.NewProviderID("github"),
		Type:           id.NewEventTypeID("PushEvent"),
		ActorLogin:     id.NewActorID("testuser"),
		ActorAvatarURL: "https://example.com/avatar.png",
		RepoName:       id.NewRepoID("owner/repo"),
		RepoURL:        "https://github.com/owner/repo",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		RawJSON:        []byte(`{"id":"12345","type":"PushEvent"}`),
	}
}

func BenchmarkToDataItem(b *testing.B) {
	item := testProviderItem()

	for range b.N {
		_ = ToDataItem(item)
	}
}

func BenchmarkFromDataItem(b *testing.B) {
	item := ToDataItem(testProviderItem())
	rawJSON := []byte(`{"id":"12345","type":"PushEvent"}`)

	for range b.N {
		_ = FromDataItem(item, rawJSON)
	}
}

func BenchmarkDataItemRoundtrip(b *testing.B) {
	item := testProviderItem()

	for range b.N {
		d := ToDataItem(item)
		_ = FromDataItem(d, item.RawJSON)
	}
}

func BenchmarkDataItemToPayload(b *testing.B) {
	item := ToDataItem(testProviderItem())

	for range b.N {
		_ = DataItemToPayload(item, []byte(`{}`))
	}
}

func BenchmarkDataItemFromPayload(b *testing.B) {
	payload := DataItemToPayload(ToDataItem(testProviderItem()), []byte(`{}`))

	for range b.N {
		_, err := DataItemFromPayload(payload)
		if err != nil {
			b.Fatal(err)
		}
	}
}
