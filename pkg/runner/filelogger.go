package runner

import (
	"fmt"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	"go.uber.org/zap"
)

// FileLogger is a logger that sends output to a file.
type FileLogger struct {
	logger *zap.Logger
}

// NewFileLogger constructs a new file logger.
func NewFileLogger(path string) *FileLogger {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{path}

	// Disable caller, level and ts output
	cfg.DisableCaller = true
	cfg.EncoderConfig.LevelKey = ""
	cfg.EncoderConfig.TimeKey = ""

	logger, _ := cfg.Build()
	return &FileLogger{logger}
}

// log an event to a file
func (fl *FileLogger) msg(idx int, id string, elapsed time.Duration, evtType eventType, message ...interface{}) {
	fl.logger.Info(fmt.Sprint(message...),
		zap.String("instanceId", id),
		zap.String("eventType", evtType.String()),
		zap.Uint64("elapsed", uint64(elapsed)),
	)
}

// log a metric to a file
func (fl *FileLogger) metric(idx int, id string, elapsed time.Duration, metric *runtime.Metric) {
	logger := fl.logger.With(
		zap.String("instanceId", id),
		zap.String("eventType", Metric.String()),
		zap.Uint64("elapsed", uint64(elapsed)),

		zap.Namespace("metric"),
		zap.String("name", metric.Name),
		zap.String("unit", metric.Unit),
		zap.Int("improve_dir", metric.ImprovementDir),
		zap.Float64("value", metric.Value),
	)

	logger.Info("")
}

func (fl *FileLogger) sync() {
	_ = fl.logger.Sync()
}
