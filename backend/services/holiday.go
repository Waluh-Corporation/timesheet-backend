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

// FetchHolidays performs an HTTP GET request to fetch Indonesia public holidays
func FetchHolidays(year, month int) ([]models.HolidayDTO, error) {
	url := fmt.Sprintf("https://api-hari-libur.vercel.app/api?year=%d", year)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []models.HolidayDTO{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("holiday API returned status code %d", resp.StatusCode)
	}

	var holidayResp models.HolidayResponse
	if err := json.NewDecoder(resp.Body).Decode(&holidayResp); err != nil {
		return nil, err
	}

	// Filter holidays that match the requested month
	prefix := fmt.Sprintf("%04d-%02d-", year, month)
	var filtered []models.HolidayDTO
	for _, hol := range holidayResp.Data {
		if strings.HasPrefix(hol.Date, prefix) {
			filtered = append(filtered, hol)
		}
	}

	return filtered, nil
}

// GetDaysInMonth calculates the number of days in the specified year and month
func GetDaysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
