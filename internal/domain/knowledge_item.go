package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// SourceType identifies the kind of official Volvo content represented by an item.
type SourceType string

const (
	SourceTypeSupportArticle      SourceType = "support_article"
	SourceTypeUserManual          SourceType = "user_manual"
	SourceTypeKnowledge           SourceType = "knowledge"
	SourceTypeSoftwareReleaseNote SourceType = "software_release_notes"
	SourceTypeQualityBulletin     SourceType = "quality_bulletin"
	SourceTypeMarketPage          SourceType = "market_page"
	SourceTypeOffer               SourceType = "offer"
	SourceTypePricing             SourceType = "pricing"
	SourceTypeCampaign            SourceType = "campaign"
)

// Valid reports whether the source type is part of the canonical v1 contract.
func (s SourceType) Valid() bool {
	switch s {
	case SourceTypeSupportArticle,
		SourceTypeUserManual,
		SourceTypeKnowledge,
		SourceTypeSoftwareReleaseNote,
		SourceTypeQualityBulletin,
		SourceTypeMarketPage,
		SourceTypeOffer,
		SourceTypePricing,
		SourceTypeCampaign:
		return true
	default:
		return false
	}
}

// KnowledgeItem is the source-independent unit stored and returned by the CLI.
type KnowledgeItem struct {
	ID            string          `json:"id"`
	SourceType    SourceType      `json:"source_type"`
	SourceID      string          `json:"source_id"`
	Market        Market          `json:"market"`
	Language      Language        `json:"language"`
	Title         string          `json:"title"`
	Body          string          `json:"body"`
	RawJSON       []byte          `json:"-"`
	Applicability []Applicability `json:"applicability,omitempty"`
	CapturedAt    time.Time       `json:"captured_at"`
	ContentHash   string          `json:"content_hash"`
	Citations     []Citation      `json:"citations,omitempty"`
}

// Valid checks the minimum invariants required before an item can be stored.
func (item KnowledgeItem) Valid() error {
	if strings.TrimSpace(item.ID) == "" {
		return errors.New("knowledge item ID is required")
	}
	if !item.SourceType.Valid() {
		return errors.New("knowledge item source type is invalid")
	}
	if strings.TrimSpace(item.SourceID) == "" {
		return errors.New("knowledge item source ID is required")
	}
	if strings.TrimSpace(string(item.Market)) == "" {
		return errors.New("knowledge item market is required")
	}
	if strings.TrimSpace(string(item.Language)) == "" {
		return errors.New("knowledge item language is required")
	}
	if item.CapturedAt.IsZero() {
		return errors.New("knowledge item captured time is required")
	}
	if strings.TrimSpace(item.ContentHash) == "" {
		return errors.New("knowledge item content hash is required")
	}
	return nil
}

// DeriveID returns the stable synthetic identity for a source item.
//
// The input contract is source type, source ID, market, then language. NUL
// separators make the encoding unambiguous while preserving that fixed order.
func DeriveID(sourceType SourceType, sourceID string, market Market, language Language) string {
	hash := sha256.New()
	for _, value := range []string{string(sourceType), sourceID, string(market), string(language)} {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// DeriveContentHash returns the stable content hash used to detect body changes.
func DeriveContentHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}
