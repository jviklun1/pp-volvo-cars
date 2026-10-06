package supportgraphql

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jviklun1/pp-volvo-cars/internal/domain"
)

var yearPattern = regexp.MustCompile(`\b(?:19|20)\d{2}\b`)
var bodyModelPattern = regexp.MustCompile(`\b[A-Z]{2,3}[0-9]{2}\b`)
var publicDocumentIDPattern = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)
var publicLocalePattern = regexp.MustCompile(`^[a-z]{2}(-[a-z]{2})?$`)

func mapDocument(value document, market domain.Market, capturedAt time.Time, matchingParagraph []string) (domain.KnowledgeItem, error) {
	body := flattenJSONBody(value.JSONContent.Body)
	if body == "" {
		body = strings.TrimSpace(value.StringContent.Description)
	}
	if value.DocumentID == "" || value.StringContent.Title == "" || body == "" {
		return domain.KnowledgeItem{}, errors.New("support document is missing ID, title, or body")
	}
	rawJSON, err := json.Marshal(value)
	if err != nil {
		return domain.KnowledgeItem{}, err
	}
	excerptValue := strings.TrimSpace(strings.Join(matchingParagraph, " "))
	if excerptValue == "" {
		excerptValue = strings.TrimSpace(value.StringContent.Description)
	}
	if excerptValue == "" {
		excerptValue = body
	}
	applicability := mapApplicability(value.ApplicableTo)
	if len(applicability) == 0 {
		applicability = mapBodyModelApplicability(value.JSONContent.Body)
	}
	item := domain.KnowledgeItem{
		SourceType:    mapDocumentType(value.DocumentType),
		SourceID:      value.DocumentID,
		Market:        canonicalMarket(market),
		Language:      domain.Language(value.Language),
		Title:         value.StringContent.Title,
		Body:          body,
		RawJSON:       rawJSON,
		Applicability: applicability,
		CapturedAt:    capturedAt.UTC(),
		Citations: []domain.Citation{{
			SourceURL: supportDocumentURL(value, market),
			Excerpt:   excerpt(excerptValue, 400), Language: domain.Language(value.Language), CapturedAt: capturedAt.UTC(),
		}},
	}
	item.ID = domain.DeriveID(item.SourceType, item.SourceID, item.Market, item.Language)
	item.ContentHash = domain.DeriveContentHash(item.Body)
	return item, nil
}

// Public topic pages use the leaf document ID, not the full ancestry ID.
// Manuals and other document kinds require additional routing context; retain
// their source reference rather than inventing a public page for them.
func supportDocumentURL(value document, market domain.Market) string {
	fallback := "volvo-support://document/" + value.DocumentID
	if value.DocumentType != "LATEST_INFO" && value.DocumentType != "SUPPORT_ARTICLE" && value.DocumentType != "SUPPORT_CONTENT" {
		return fallback
	}
	if !publicDocumentIDPattern.MatchString(value.DocumentID) {
		return fallback
	}
	locale := upstreamMarket(market)
	switch locale {
	case "be":
		locale = "nl-be"
		if strings.HasPrefix(value.Language, "fr") {
			locale = "fr-be"
		}
	case "ca":
		locale = "en-ca"
		if strings.HasPrefix(value.Language, "fr") {
			locale = "fr-ca"
		}
	case "ch":
		locale = "de-ch"
		if strings.HasPrefix(value.Language, "fr") {
			locale = "fr-ch"
		} else if strings.HasPrefix(value.Language, "it") {
			locale = "it-ch"
		}
	}
	if !publicLocalePattern.MatchString(locale) {
		return fallback
	}
	return "https://www.volvocars.com/" + locale + "/support/topic/" + value.DocumentID + "/"
}

func mapBodyModelApplicability(raw json.RawMessage) []domain.Applicability {
	var root any
	if json.Unmarshal(raw, &root) != nil {
		return nil
	}
	models := make(map[domain.ModelSlug]struct{})
	var visit func(any)
	visit = func(current any) {
		switch typed := current.(type) {
		case []any:
			for _, child := range typed {
				visit(child)
			}
		case map[string]any:
			if typed["type"] == "subSegment" {
				children, _ := typed["children"].([]any)
				if len(children) > 0 && strings.EqualFold(flattenValue(children[0]), "Applicable models") {
					for _, child := range children[1:] {
						for _, match := range bodyModelPattern.FindAllString(flattenValue(child), -1) {
							models[domain.ModelSlug(strings.ToLower(match))] = struct{}{}
						}
					}
				}
			}
			if children, ok := typed["children"]; ok {
				visit(children)
			}
		}
	}
	visit(root)
	values := make([]string, 0, len(models))
	for model := range models {
		values = append(values, string(model))
	}
	sort.Strings(values)
	result := make([]domain.Applicability, 0, len(values))
	for _, rawModel := range values {
		model := domain.ModelSlug(rawModel)
		result = append(result, domain.Applicability{ModelSlug: &model})
	}
	return result
}

func flattenValue(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return flattenJSONBody(encoded)
}

func mapDocumentType(value string) domain.SourceType {
	switch value {
	case "USER_MANUAL", "QUICK_GUIDE":
		return domain.SourceTypeUserManual
	case "QUALITY_BULLETIN":
		return domain.SourceTypeQualityBulletin
	case "SOFTWARE_RELEASE_NOTES", "SOFTWARE_RELEASE_NOTES_DIFF":
		return domain.SourceTypeSoftwareReleaseNote
	case "KNOWLEDGE", "CONTEXTUAL_KNOWLEDGE", "SERVICE_AND_WARRANTY":
		return domain.SourceTypeKnowledge
	default:
		return domain.SourceTypeSupportArticle
	}
}

func flattenJSONBody(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	var parts []string
	var visit func(any)
	visit = func(current any) {
		switch typed := current.(type) {
		case string:
			if value := strings.TrimSpace(typed); value != "" {
				parts = append(parts, value)
			}
		case []any:
			for _, child := range typed {
				visit(child)
			}
		case map[string]any:
			if children, ok := typed["children"]; ok {
				visit(children)
			}
		}
	}
	visit(value)
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}

func mapApplicability(values []applicableTo) []domain.Applicability {
	var models []*domain.ModelSlug
	var years []*domain.ModelYear
	var powertrains []*domain.Powertrain
	for _, group := range values {
		title := strings.ToLower(group.Title)
		for _, rawValue := range group.Values {
			switch {
			case strings.Contains(title, "year"):
				match := yearPattern.FindString(rawValue)
				if match == "" {
					continue
				}
				year, _ := strconv.Atoi(match)
				value := domain.ModelYear(year)
				years = append(years, &value)
			case strings.Contains(title, "model"):
				value := domain.ModelSlug(strings.ToLower(strings.TrimSpace(rawValue)))
				models = append(models, &value)
			case strings.Contains(title, "powertrain") || strings.Contains(title, "engine"):
				value := domain.Powertrain(strings.ToLower(strings.TrimSpace(rawValue)))
				powertrains = append(powertrains, &value)
			}
		}
	}
	if len(models) == 0 && len(years) == 0 && len(powertrains) == 0 {
		return nil
	}
	if len(models) == 0 {
		models = []*domain.ModelSlug{nil}
	}
	if len(years) == 0 {
		years = []*domain.ModelYear{nil}
	}
	if len(powertrains) == 0 {
		powertrains = []*domain.Powertrain{nil}
	}
	result := make([]domain.Applicability, 0, len(models)*len(years)*len(powertrains))
	for _, model := range models {
		for _, year := range years {
			for _, powertrain := range powertrains {
				result = append(result, domain.Applicability{ModelSlug: model, ModelYear: year, Powertrain: powertrain})
			}
		}
	}
	return result
}

func canonicalMarket(market domain.Market) domain.Market {
	if market == "uk" {
		return "gb"
	}
	return market
}

func upstreamMarket(market domain.Market) string {
	if canonicalMarket(market) == "gb" {
		return "uk"
	}
	return string(market)
}

func resultVersion(query string, items []domain.KnowledgeItem) string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, item.ID+":"+item.ContentHash)
	}
	sort.Strings(values)
	sum := sha256.Sum256([]byte(query + "\x00" + strings.Join(values, "\x00")))
	return hex.EncodeToString(sum[:])
}

func excerpt(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}
