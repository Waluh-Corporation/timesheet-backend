package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"timesheet-backend/config"
)

// DocusealSubmitterValue represents a single filled field value returned by DocuSeal.
type DocusealSubmitterValue struct {
	Field string `json:"field"`
	Value any    `json:"value"`
}

// DocusealDocumentItem represents a signed document reference in DocuSeal responses.
type DocusealDocumentItem struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// DocusealSubmitterResponse represents a submitter object returned by DocuSeal REST API.
type DocusealSubmitterResponse struct {
	ID           int64                    `json:"id"`
	SubmissionID int64                    `json:"submission_id"`
	UUID         string                   `json:"uuid"`
	Email        string                   `json:"email"`
	Slug         string                   `json:"slug"`
	Name         string                   `json:"name"`
	Status       string                   `json:"status"`
	Role         string                   `json:"role"`
	ExternalID   string                   `json:"external_id"`
	EmbedSrc     string                   `json:"embed_src"`
	CompletedAt  *time.Time               `json:"completed_at"`
	Values       []DocusealSubmitterValue `json:"values"`
	Documents    []DocusealDocumentItem   `json:"documents"`
}

// CreateSubmissionSubmitterReq defines one submitter entry when creating a DocuSeal submission.
type CreateSubmissionSubmitterReq struct {
	Name       string            `json:"name,omitempty"`
	Email      string            `json:"email,omitempty"`
	Role       string            `json:"role,omitempty"`
	ExternalID string            `json:"external_id,omitempty"`
	SendEmail  *bool             `json:"send_email,omitempty"`
	Values     map[string]any    `json:"values,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// CreateSubmissionReq defines the request body for POST /api/submissions.
type CreateSubmissionReq struct {
	TemplateID int                            `json:"template_id"`
	SendEmail  bool                           `json:"send_email"`
	Submitters []CreateSubmissionSubmitterReq `json:"submitters"`
}

// DocusealClient defines the contract for communicating with a self-hosted DocuSeal server.
type DocusealClient interface {
	IsEnabled() bool
	CreateSpecimenSubmission(ctx context.Context, userID uint, name, email, employeeID, division string) (*DocusealSubmitterResponse, error)
	GetSubmitter(ctx context.Context, submitterID int64) (*DocusealSubmitterResponse, error)
	DownloadAsset(ctx context.Context, assetURL string) ([]byte, error)
	RewriteEmbedURL(slug string, rawEmbedSrc string) string
}

type docusealHTTPClient struct {
	enabled            bool
	internalURL        string
	publicURL          string
	apiKey             string
	specimenTemplateID int
	signatureFieldName string
	httpClient         *http.Client
}

// NewDocusealClient constructs a DocusealClient configured from config.Config.
func NewDocusealClient(cfg *config.Config) DocusealClient {
	if cfg == nil {
		return &docusealHTTPClient{}
	}
	fieldName := strings.TrimSpace(cfg.DocusealSignatureFieldName)
	if fieldName == "" {
		fieldName = "Signature"
	}
	return &docusealHTTPClient{
		enabled:            cfg.DocusealEnabled && cfg.DocusealAPIKey != "" && cfg.DocusealInternalURL != "",
		internalURL:        strings.TrimRight(strings.TrimSpace(cfg.DocusealInternalURL), "/"),
		publicURL:          strings.TrimRight(strings.TrimSpace(cfg.DocusealPublicURL), "/"),
		apiKey:             strings.TrimSpace(cfg.DocusealAPIKey),
		specimenTemplateID: cfg.DocusealSpecimenTemplateID,
		signatureFieldName: fieldName,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *docusealHTTPClient) IsEnabled() bool {
	return c != nil && c.enabled && c.specimenTemplateID > 0
}

func (c *docusealHTTPClient) apiBaseURL() string {
	base := c.internalURL
	if strings.HasSuffix(base, "/api") {
		return base
	}
	return base + "/api"
}

// RewriteEmbedURL normalizes a DocuSeal embed_src URL so browsers always load from DOCUSEAL_PUBLIC_URL.
func (c *docusealHTTPClient) RewriteEmbedURL(slug string, rawEmbedSrc string) string {
	pub := c.publicURL
	if pub == "" {
		pub = c.internalURL
	}
	pub = strings.TrimSuffix(pub, "/api")
	if slug != "" && pub != "" {
		return fmt.Sprintf("%s/s/%s", pub, slug)
	}
	if rawEmbedSrc == "" || pub == "" {
		return rawEmbedSrc
	}
	parsed, err := url.Parse(rawEmbedSrc)
	if err != nil {
		return rawEmbedSrc
	}
	pubParsed, err := url.Parse(pub)
	if err != nil {
		return rawEmbedSrc
	}
	parsed.Scheme = pubParsed.Scheme
	parsed.Host = pubParsed.Host
	return parsed.String()
}

// RewriteAssetToInternalURL rewrites a public or relative DocuSeal blob URL to use DOCUSEAL_INTERNAL_URL
// so the backend downloads PNGs over the local network when self-hosted.
func (c *docusealHTTPClient) RewriteAssetToInternalURL(assetURL string) string {
	trimmed := strings.TrimSpace(assetURL)
	if trimmed == "" || c.internalURL == "" {
		return trimmed
	}
	internalBase := strings.TrimSuffix(c.internalURL, "/api")
	if strings.HasPrefix(trimmed, "/") {
		return internalBase + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	intParsed, err := url.Parse(internalBase)
	if err != nil {
		return trimmed
	}
	parsed.Scheme = intParsed.Scheme
	parsed.Host = intParsed.Host
	return parsed.String()
}

func (c *docusealHTTPClient) CreateSpecimenSubmission(ctx context.Context, userID uint, name, email, employeeID, division string) (*DocusealSubmitterResponse, error) {
	if !c.IsEnabled() {
		return nil, fmt.Errorf("docuseal integration is not enabled or template ID is not configured")
	}

	shortName := FormatSignatureShortName(name)
	if shortName == "" {
		shortName = name
	}

	sendEmail := false
	reqPayload := CreateSubmissionReq{
		TemplateID: c.specimenTemplateID,
		SendEmail:  false,
		Submitters: []CreateSubmissionSubmitterReq{
			{
				Name:       shortName,
				Email:      email,
				ExternalID: fmt.Sprintf("user-%d", userID),
				SendEmail:  &sendEmail,
				Values: map[string]any{
					"Name":     name,
					"Nama":     name,
					"NPP":      employeeID,
					"Division": division,
					"Divisi":   division,
				},
				Metadata: map[string]string{
					"user_id": strconv.FormatUint(uint64(userID), 10),
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal docuseal submission request: %w", err)
	}

	endpoint := c.apiBaseURL() + "/submissions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create docuseal submission request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Auth-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute docuseal submission request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	const maxRespSize = 5 * 1024 * 1024
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxRespSize))
	if err != nil {
		return nil, fmt.Errorf("read docuseal submission response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docuseal POST /submissions returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// DocuSeal POST /submissions returns either an array of submitters or a submission object containing submitters
	var submitters []DocusealSubmitterResponse
	if err := json.Unmarshal(respBody, &submitters); err == nil && len(submitters) > 0 {
		sub := submitters[0]
		sub.EmbedSrc = c.RewriteEmbedURL(sub.Slug, sub.EmbedSrc)
		return &sub, nil
	}

	var wrapped struct {
		ID         int64                       `json:"id"`
		Submitters []DocusealSubmitterResponse `json:"submitters"`
	}
	if err := json.Unmarshal(respBody, &wrapped); err != nil || len(wrapped.Submitters) == 0 {
		return nil, fmt.Errorf("unexpected docuseal submission response payload: %s", string(respBody))
	}

	sub := wrapped.Submitters[0]
	if sub.SubmissionID == 0 {
		sub.SubmissionID = wrapped.ID
	}
	sub.EmbedSrc = c.RewriteEmbedURL(sub.Slug, sub.EmbedSrc)
	return &sub, nil
}

func (c *docusealHTTPClient) GetSubmitter(ctx context.Context, submitterID int64) (*DocusealSubmitterResponse, error) {
	if c == nil || c.internalURL == "" || c.apiKey == "" {
		return nil, fmt.Errorf("docuseal client is not configured")
	}

	endpoint := fmt.Sprintf("%s/submitters/%d", c.apiBaseURL(), submitterID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create get submitter request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Auth-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute get submitter request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	const maxRespSize = 5 * 1024 * 1024
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxRespSize))
	if err != nil {
		return nil, fmt.Errorf("read get submitter response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docuseal GET /submitters/%d returned status %d: %s", submitterID, resp.StatusCode, string(respBody))
	}

	var sub DocusealSubmitterResponse
	if err := json.Unmarshal(respBody, &sub); err != nil {
		return nil, fmt.Errorf("unmarshal submitter response: %w", err)
	}
	sub.EmbedSrc = c.RewriteEmbedURL(sub.Slug, sub.EmbedSrc)
	return &sub, nil
}

func (c *docusealHTTPClient) DownloadAsset(ctx context.Context, assetURL string) ([]byte, error) {
	targetURL := c.RewriteAssetToInternalURL(assetURL)
	if targetURL == "" {
		return nil, fmt.Errorf("empty asset URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create asset download request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("X-Auth-Token", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download asset from docuseal: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docuseal asset download returned status %d", resp.StatusCode)
	}

	const maxAssetBytes = 15 * 1024 * 1024 // 15 MiB ceiling
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAssetBytes))
	if err != nil {
		return nil, fmt.Errorf("read asset bytes: %w", err)
	}
	return data, nil
}

// VerifyDocusealWebhookSignature validates the X-Docuseal-Signature header against rawBody
// using HMAC-SHA256 (supporting both "[timestamp].[signature]" with a 5-minute window and direct hex/sha256= signatures).
func VerifyDocusealWebhookSignature(sigHeader string, rawBody []byte, secret string, now time.Time) bool {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return false
	}
	sigHeader = strings.TrimSpace(sigHeader)
	if sigHeader == "" {
		return false
	}

	if tsStr, sigHex, ok := strings.Cut(sigHeader, "."); ok && tsStr != "" && sigHex != "" {
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			return false
		}
		if math.Abs(float64(now.Unix()-ts)) > 300 {
			return false
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(tsStr))
		_, _ = mac.Write([]byte("."))
		_, _ = mac.Write(rawBody)
		expectedHex := hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(strings.ToLower(expectedHex)), []byte(strings.ToLower(sigHex)))
	}

	cleanSig := strings.TrimPrefix(strings.ToLower(sigHeader), "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(rawBody)
	expectedHex := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(strings.ToLower(expectedHex)), []byte(cleanSig))
}

// ExtractSignatureValue inspects a submitter's field values and extracts either a signature image URL/data-URI
// or a typed signature text string.
func ExtractSignatureValue(values []DocusealSubmitterValue, preferredFieldName string) (rawVal string, isImageURL bool, isDataURI bool) {
	var fallbackText string
	prefLower := strings.ToLower(strings.TrimSpace(preferredFieldName))

	for _, v := range values {
		strVal, ok := v.Value.(string)
		if !ok {
			continue
		}
		strVal = strings.TrimSpace(strVal)
		if strVal == "" {
			continue
		}

		fieldLower := strings.ToLower(strings.TrimSpace(v.Field))
		isSigField := (prefLower != "" && fieldLower == prefLower) ||
			strings.Contains(fieldLower, "signature") ||
			strings.Contains(fieldLower, "tanda tangan") ||
			strings.Contains(fieldLower, "ttd")

		if strings.HasPrefix(strVal, "data:image/") {
			return strVal, false, true
		}
		if isLikelySignatureAssetURL(strVal) {
			return strVal, true, false
		}
		if isSigField && fallbackText == "" {
			fallbackText = strVal
		}
	}

	if fallbackText != "" {
		return fallbackText, false, false
	}

	// If the template had only 1 field and its name wasn't "Signature", check the first non-empty string value
	if len(values) == 1 {
		if strVal, ok := values[0].Value.(string); ok && strings.TrimSpace(strVal) != "" {
			return strings.TrimSpace(strVal), false, false
		}
	}
	return "", false, false
}

func isLikelySignatureAssetURL(val string) bool {
	if !strings.HasPrefix(val, "http://") && !strings.HasPrefix(val, "https://") && !strings.HasPrefix(val, "/blobs/") && !strings.HasPrefix(val, "/file/") {
		return false
	}
	lower := strings.ToLower(val)
	return strings.HasSuffix(lower, ".png") ||
		strings.HasSuffix(lower, ".jpg") ||
		strings.HasSuffix(lower, ".jpeg") ||
		strings.Contains(lower, "/blobs/") ||
		strings.Contains(lower, "signature")
}
