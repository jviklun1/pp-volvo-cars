package domain

import (
	"testing"
	"time"
)

func TestDeriveIDIsStableAndUnambiguous(t *testing.T) {
	want := "f92bc2a3edb65d2f877fa5a9f6febeff9d18c798dc1d8b7e279a36b370cc1f32"
	got := DeriveID(SourceTypeSupportArticle, "article-123", Market("se"), Language("sv"))
	if got != want {
		t.Fatalf("DeriveID() = %q, want %q", got, want)
	}

	left := DeriveID(SourceType("a"), "bc", Market("d"), Language("e"))
	right := DeriveID(SourceType("ab"), "c", Market("d"), Language("e"))
	if left == right {
		t.Fatal("component boundaries must affect the derived ID")
	}
}

func TestKnowledgeItemValid(t *testing.T) {
	item := KnowledgeItem{
		ID:          DeriveID(SourceTypeSupportArticle, "article-123", Market("se"), Language("sv")),
		SourceType:  SourceTypeSupportArticle,
		SourceID:    "article-123",
		Market:      Market("se"),
		Language:    Language("sv"),
		CapturedAt:  time.Now(),
		ContentHash: DeriveContentHash("body"),
	}
	if err := item.Valid(); err != nil {
		t.Fatalf("Valid() returned an unexpected error: %v", err)
	}
}
