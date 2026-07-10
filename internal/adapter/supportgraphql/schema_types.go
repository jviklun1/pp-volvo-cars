package supportgraphql

import "encoding/json"

type marketsData struct {
	Markets []struct {
		ID      string `json:"id"`
		Caption string `json:"caption"`
	} `json:"markets"`
}

type modelsData struct {
	Market *struct {
		CarsByDisplayName []struct {
			DisplayName string `json:"displayName"`
			ModelSlug   string `json:"modelSlug"`
		} `json:"carsByDisplayName"`
	} `json:"market"`
}

type searchData struct {
	Market *struct {
		Search         searchResponse `json:"search"`
		CarByModelSlug *struct {
			Search searchResponse `json:"search"`
		} `json:"carByModelSlug"`
	} `json:"market"`
}

type searchResponse struct {
	Results []documentSearchResult `json:"results"`
}

type documentSearchResult struct {
	Score             *float64 `json:"score"`
	MatchingParagraph []string `json:"matchingParagraph"`
	Document          document `json:"document"`
}

type documentData struct {
	Market *struct {
		Document *document `json:"document"`
	} `json:"market"`
}

type document struct {
	DocumentID     string         `json:"documentId"`
	ExternalID     string         `json:"externalId"`
	FullDocumentID string         `json:"fullDocumentId"`
	Language       string         `json:"language"`
	DocumentType   string         `json:"documentType"`
	LastUpdate     string         `json:"lastUpdate"`
	StringContent  stringContent  `json:"stringContent"`
	JSONContent    documentJSON   `json:"jsonContent"`
	ApplicableTo   []applicableTo `json:"applicableTo"`
}

type stringContent struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type documentJSON struct {
	Body                     json.RawMessage `json:"body"`
	JSONContentSchemaVersion string          `json:"jsonContentSchemaVersion"`
}

type applicableTo struct {
	Title  string   `json:"title"`
	Values []string `json:"values"`
}
