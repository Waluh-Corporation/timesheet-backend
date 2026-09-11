package services_test

import (
	"testing"
	"time"

	"timesheet-backend/models"
	"timesheet-backend/services"
)

func sampleBenchmarkInput(company string) services.GenerationInput {
	var activities []models.DailyActivity
	for i := 1; i <= 22; i++ {
		activities = append(activities, models.DailyActivity{
			Date:        time.Date(2026, 7, i, 0, 0, 0, 0, time.UTC),
			StartTime:   "08:00",
			EndTime:     "17:00",
			Status:      "P",
			Activity:    "Developed backend API and unit tests",
			ProjectName: "BNI Direct Cash",
			ProjectID:   "P24015",
			AppImpacted: "BNI Direct",
		})
	}

	return services.GenerationInput{
		CompanyCode: company,
		Year:        2026,
		Month:       7,
		User:        &models.User{Name: "John Benchmark", EmployeeID: "MII12345", Division: "Digital Banking", Department: "Delivery"},
		Holidays:    map[int]string{17: "Hari Kemerdekaan"},
		Activities:  activities,
	}
}

func BenchmarkBuildMIIWorkbook(b *testing.B) {
	in := sampleBenchmarkInput("mii")
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := services.GenerateFromTemplate(in)
		if err != nil {
			b.Fatalf("generation failed: %v", err)
		}
	}
}

func BenchmarkBuildNTTWorkbook(b *testing.B) {
	in := sampleBenchmarkInput("ntt")
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := services.GenerateFromTemplate(in)
		if err != nil {
			b.Fatalf("generation failed: %v", err)
		}
	}
}
