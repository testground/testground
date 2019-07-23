package tpipeline

import "github.com/ipfs/test-pipeline/store"

type MetricImprovementDir int

const (
	ImproveDirUp   MetricImprovementDir = 0
	ImproveDirDown MetricImprovementDir = 1
)

type MetricID string

// EnsureMetricOpts encapsulates arguments to the EnsureMetric operation.
type EnsureMetricOpts struct {
	Name           string
	Description    string
	Unit           string
	TestCase       string
	ImprovementDir MetricImprovementDir
	UIOrder        *int // nillable
}

// RecordResultOpts encapsulates arguments to the RecordResult operation.
type RecordResultOpts struct {
	ID            MetricID
	TestRun       int
	TestIteration int
	Value         float64
}

type QueryResultsOpts struct {
	// TBD
}

type QueryResultResult struct {
	// TBD
}

// ReportingService is the ingestion point for all quantifiable observations produced
// by test cases.
type ReportingService interface {
	// EnsureMetric performs checks to verify that a metric is enlisted in the catalogue.
	// It returns the unique ID that identifies the metric, so it can be used when recording
	// metrics via RecordResult.
	EnsureMetric(opts *EnsureMetricOpts) (MetricID, error)

	// RecordResult allows a test case to record an observation about a metric.
	RecordResult(opts *RecordResultOpts) error

	// QueryResults fetches results, filtering by the provided options and returning
	// matching metrics.
	QueryResults(opts *QueryResultsOpts) ([]*QueryResultResult, error)
}

func NewReportingService(db *store.SqliteStore, cfg *Config) ReportingService {
	s := &dbReportingService{db, cfg}
	s.Register()
	return s
}

type dbReportingService struct {
	DB     *store.SqliteStore
	Config *Config
}

var _ ReportingService = (*dbReportingService)(nil)

func (r *dbReportingService) EnsureMetric(opts *EnsureMetricOpts) (MetricID, error) {
	panic("implement me")
}

func (r *dbReportingService) RecordResult(opts *RecordResultOpts) error {
	panic("implement me")
}

func (r *dbReportingService) QueryResults(opts *QueryResultsOpts) ([]*QueryResultResult, error) {
	panic("implement me")
}

func (r *dbReportingService) Register() {
	// TODO: Register registers handlers to service HTTP requests.
}

func (r *dbReportingService) Close() error {
	return nil
}
