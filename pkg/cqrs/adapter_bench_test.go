package cqrs

import (
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

func testProviderItem() *provider.Item {
	return &provider.Item{
		ID:         id.NewItemID(),
		SourceID: id.NewSourceID("12345"),
		Source:     id.NewProviderID("github"),
		Type:       id.NewEventTypeID("PushEvent"),
		Attributes: map[string]string{
			"actor_login":      "testuser",
			"actor_avatar_url": "https://example.com/avatar.png",
			"repo_name":        "owner/repo",
			"repo_url":         "https://github.com/owner/repo",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		RawJSON:   []byte(`{"id":"12345","type":"PushEvent"}`),
	}
}

func BenchmarkToDataItem(b *testing.B) {
	item := testProviderItem()
	for range b.N {
		_ = toDataItem(item)
	}
}

func BenchmarkDataItemToPayload(b *testing.B) {
	item := toDataItem(testProviderItem())
	for range b.N {
		_ = dataItemToPayload(item, []byte(`{}`))
	}
}

func BenchmarkDataItemFromPayload(b *testing.B) {
	payload := dataItemToPayload(toDataItem(testProviderItem()), []byte(`{}`))
	for range b.N {
		_, err := dataItemFromPayload(payload)
		if err != nil {
			b.Fatal(err)
		}
	}
}
