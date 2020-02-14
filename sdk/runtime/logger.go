package runtime

import (
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type logger struct {
	runenv *RunParams

	// TODO: we'll want different kinds of loggers.
	logger  *zap.Logger
	slogger *zap.SugaredLogger
}

func newLogger(runenv *RunParams) *logger {
	l := &logger{runenv: runenv}
	l.init()
	return l
}

func (l *logger) init() {
	level := zap.NewAtomicLevel()

	if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
		if err := level.UnmarshalText([]byte(lvl)); err != nil {
			defer func() {
				// once the logger is defined...
				if l.slogger != nil {
					l.slogger.Errorf("failed to decode log level '%q': %s", l, err)
				}
			}()
		}
	} else {
		level.SetLevel(zapcore.InfoLevel)
	}

	paths := []string{"stdout"}
	if l.runenv.TestOutputsPath != "" {
		paths = append(paths, filepath.Join(l.runenv.TestOutputsPath, "run.out"))
	}

	cfg := zap.Config{
		Development:       false,
		Level:             level,
		DisableCaller:     true,
		DisableStacktrace: true,
		OutputPaths:       paths,
		Encoding:          "json",
		InitialFields: map[string]interface{}{
			"run_id":   l.runenv.TestRun,
			"group_id": l.runenv.TestGroupID,
		},
	}

	enc := zap.NewProductionEncoderConfig()
	enc.LevelKey, enc.NameKey = "", ""
	enc.EncodeTime = zapcore.EpochNanosTimeEncoder
	cfg.EncoderConfig = enc

	var err error
	maxAttempts := 5
	for i := 0; i < maxAttempts; i++ {
		l.logger, err = cfg.Build()
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		panic(err)
	}

	l.slogger = l.logger.Sugar()
}

func (l *logger) SLogger() *zap.SugaredLogger {
	return l.slogger
}

// Loggers returns the loggers populated from this runenv.
func (l *logger) Loggers() (*zap.Logger, *zap.SugaredLogger) {
	return l.logger, l.slogger
}
