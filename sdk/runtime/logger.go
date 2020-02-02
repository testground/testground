package runtime

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type logger struct {
	runenv *RunEnv

	// TODO: we'll want different kinds of loggers.
	logger  *zap.Logger
	slogger *zap.SugaredLogger
}

func newLogger(runenv *RunEnv) *logger {
	l := &logger{runenv: runenv}
	l.init()
	return l
}

func (l *logger) init() {
	level := zap.NewAtomicLevel()
	if l := os.Getenv("LOG_LEVEL"); l != "" {
		level.UnmarshalText([]byte(l))
	} else {
		level.SetLevel(zapcore.InfoLevel)
	}

	cfg := zap.Config{
		Development:       false,
		Level:             level,
		DisableCaller:     true,
		DisableStacktrace: true,
		OutputPaths:       []string{},
		ErrorOutputPaths:  []string{},
		Encoding:          "json",
		InitialFields: map[string]interface{}{
			"run_id":   l.runenv.TestRun,
			"group_id": l.runenv.TestGroupID,
		},
	}

	var err error
	l.logger, err = cfg.Build()
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
