package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"timesheet-backend/models"
)

const (
	kemendesaBaseURL = "https://api.kemendesa.link/libur-nasional/api/holidays"
)

// FetchHolidaysByYear queries the Kemendesa API for all national holidays and joint leave in a given year.
func FetchHolidaysByYear(year int) ([]models.HolidayDTO, error) {
	url := fmt.Sprintf("%s/%d.json", kemendesaBaseURL, year)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TimesheetGenerator/1.0 (Kemendesa Holiday Client)")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// If year-specific endpoint returns 404, attempt fallback to /latest
	if resp.StatusCode == http.StatusNotFound {
		latestURL := fmt.Sprintf("%s/latest", kemendesaBaseURL)
		latestReq, lErr := http.NewRequestWithContext(ctx, "GET", latestURL, nil)
		if lErr == nil {
			latestReq.Header.Set("User-Agent", "TimesheetGenerator/1.0 (Kemendesa Holiday Client)")
			latestReq.Header.Set("Accept", "application/json")
			if latestResp, doErr := client.Do(latestReq); doErr == nil {
				defer latestResp.Body.Close()
				if latestResp.StatusCode == http.StatusOK {
					var latestData models.KemendesaHolidayResponse
					if decErr := json.NewDecoder(latestResp.Body).Decode(&latestData); decErr == nil {
						if latestData.Metadata.Year == year {
							return mapKemendesaToDTO(latestData.Data), nil
						}
					}
				}
			}
		}
		return []models.HolidayDTO{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kemendesa holiday API returned status code %d", resp.StatusCode)
	}

	var holidayResp models.KemendesaHolidayResponse
	if err := json.NewDecoder(resp.Body).Decode(&holidayResp); err != nil {
		return nil, fmt.Errorf("failed to decode kemendesa response: %w", err)
	}

	return mapKemendesaToDTO(holidayResp.Data), nil
}

// FetchHolidays performs an HTTP GET request to fetch Indonesia public holidays for a given month and year
func FetchHolidays(year, month int) ([]models.HolidayDTO, error) {
	yearlyHolidays, err := FetchHolidaysByYear(year)
	if err != nil {
		return nil, err
	}

	// Filter holidays that match the requested month (YYYY-MM-)
	prefix := fmt.Sprintf("%04d-%02d-", year, month)
	var filtered []models.HolidayDTO
	for _, hol := range yearlyHolidays {
		if strings.HasPrefix(hol.Date, prefix) {
			filtered = append(filtered, hol)
		}
	}

	return filtered, nil
}

func mapKemendesaToDTO(items []models.KemendesaHolidayItem) []models.HolidayDTO {
	result := make([]models.HolidayDTO, 0, len(items))
	for _, item := range items {
		result = append(result, models.HolidayDTO{
			Date:          item.Date,
			Description:   item.Name,
			IsJointLeave:  item.IsCutiBersama,
			IsCutiBersama: item.IsCutiBersama,
			IsCivic:       item.IsCivic,
			IsReligious:   item.IsReligious,
		})
	}
	return result
}

// GetDaysInMonth calculates the number of days in the specified year and month
func GetDaysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
