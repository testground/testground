package store

import (
	"time"

	"github.com/jinzhu/gorm"
)

type TestPlan struct {
	gorm.Model

	Desc   string `gorm:"not null"`
	Runner int
}

type TestCase struct {
	gorm.Model

	Desc       string
	TestPlanID uint
	TestPlan   TestPlan
}

// TestRun represents a test run spawned by the system.
type TestRun struct {
	gorm.Model

	TestPlanID      uint
	TestPlan        TestPlan
	CommitHash      string
	Reason          string
	TotalIterations int
	StartedAt       time.Time
	EndedAt         *time.Time
}

type TestIteration struct {
	gorm.Model

	TestRunID uint
	TestRun   TestRun
	Number    int
	StartedAt time.Time
	EndedAt   *time.Time
}

type MetricDefinition struct {
	gorm.Model

	Name       string
	Desc       string
	Unit       string
	ImproveDir int
	UIOrder    int
	TestCaseID uint
	TestCase   TestCase
}

type Result struct {
	gorm.Model

	MetricID        uint
	Metric          MetricDefinition
	TestIterationID uint
	TestIteration   TestIteration
	Value           float64
	RecordedAt      time.Time
}

type Commit struct {
	SHA        string `gorm:"primary_key"`
	RepoURL    string
	Author     string
	Branch     string
	Message    string
	CommitedAt time.Time
}
