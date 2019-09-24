package logging

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger  *zap.Logger
	sugared *zap.SugaredLogger
)

func init() {
	var err error
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)

	if level := os.Getenv("LOG_LEVEL"); level != "" {
		l := zapcore.Level(0)
		l.UnmarshalText([]byte(level))
		cfg.Level = zap.NewAtomicLevelAt(l)
	}

	logger, err = cfg.Build()
	if err != nil {
		panic(err)
	}
	sugared = logger.Sugar()
}

// L returns the global raw logger.
func L() *zap.Logger {
	return logger
}

// S returns the global sugared logger.
func S() *zap.SugaredLogger {
	return sugared
}
