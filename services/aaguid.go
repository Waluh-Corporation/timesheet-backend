package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"timesheet-backend/database"
	"timesheet-backend/models"
)

// DefaultCommunityAAGUIDURL is the official community catalog URL.
const DefaultCommunityAAGUIDURL = "https://raw.githubusercontent.com/passkeydeveloper/passkey-authenticator-aaguids/main/aaguid.json"

// CommunityAAGUIDURL is the endpoint URL queried for community authenticators.
// Exported as a variable to allow overriding in unit tests.
var CommunityAAGUIDURL = DefaultCommunityAAGUIDURL

// CommunityAAGUIDEntry represents the JSON schema of each authenticator in the community catalog.
type CommunityAAGUIDEntry struct {
	Name string `json:"name"`
	Icon string `json:"icon_light"`
}

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// FetchCommunityAAGUIDs retrieves the community AAGUID registry JSON from GitHub.
func FetchCommunityAAGUIDs(ctx context.Context) (map[string]CommunityAAGUIDEntry, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, CommunityAAGUIDURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("User-Agent", "TimesheetBackend-AAGUID-Sync/1.0 (passkey-authenticator-sync)")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("community registry returned unexpected HTTP status %d", resp.StatusCode)
	}

	// Limit response reading to 15MB to prevent memory exhaustion attacks
	limitedReader := io.LimitReader(resp.Body, 15*1024*1024)

	var rawData map[string]CommunityAAGUIDEntry
	if err := json.NewDecoder(limitedReader).Decode(&rawData); err != nil {
		return nil, fmt.Errorf("failed to parse community aaguid JSON: %w", err)
	}

	result := make(map[string]CommunityAAGUIDEntry, len(rawData))
	for k, v := range rawData {
		normalizedKey := strings.ToLower(strings.TrimSpace(k))
		if !uuidRegex.MatchString(normalizedKey) {
			continue // skip malformed keys
		}
		if strings.TrimSpace(v.Name) == "" {
			continue // skip unnamed entries
		}
		result[normalizedKey] = v
	}

	return result, nil
}

// SyncCommunityAAGUIDsToDB fetches authenticators from the community registry,
// performs batch upserts into the database, and refreshes the in-memory registry.
func SyncCommunityAAGUIDsToDB(ctx context.Context, db *gorm.DB) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("database connection is nil")
	}

	catalog, err := FetchCommunityAAGUIDs(ctx)
	if err != nil {
		return 0, err
	}

	if len(catalog) == 0 {
		return 0, fmt.Errorf("no valid authenticators found in community registry")
	}

	records := make([]models.AuthenticatorAAGUID, 0, len(catalog))
	now := time.Now()

	for aaguid, entry := range catalog {
		records = append(records, models.AuthenticatorAAGUID{
			AAGUID:    aaguid,
			Name:      strings.TrimSpace(entry.Name),
			Icon:      strings.TrimSpace(entry.Icon),
			UpdatedAt: now,
		})
	}

	// Upsert in batches of 50
	const batchSize = 50
	err = db.Transaction(func(tx *gorm.DB) error {
		for i := 0; i < len(records); i += batchSize {
			end := i + batchSize
			if end > len(records) {
				end = len(records)
			}
			batch := records[i:end]

			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "aaguid"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"name",
					"icon",
					"updated_at",
				}),
			}).Create(&batch).Error; err != nil {
				return fmt.Errorf("failed to upsert aaguid batch: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	// Immediately reload into in-memory cache
	if err := database.SyncAuthenticatorAAGUIDs(db); err != nil {
		return len(records), fmt.Errorf("database updated but failed to reload in-memory cache: %w", err)
	}

	return len(records), nil
}
