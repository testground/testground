package logging

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger  *zap.Logger
	sugared *zap.SugaredLogger

	level = zap.NewAtomicLevelAt(zapcore.WarnLevel)

	terminal = false
)

func init() {
	DevelopmentMode()
}

// IsTerminal returns whether we're running in terminal mode.
func IsTerminal() bool {
	return terminal
}

// SetLevel adjusts the level of the loggers.
func SetLevel(l zapcore.Level) {
	level.SetLevel(l)
}

// TerminalMode switches logging output to TTY mode.
func TerminalMode() {
	terminal = true

	cfg := zap.NewDevelopmentConfig()
	cfg.Level = level
	cfg.DisableCaller = true
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.UTC().Format(time.StampMicro))
	}

	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	sugared = logger.Sugar()
}

// DevelopmentMode switches logging output to development mode.
func DevelopmentMode() {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = level

	logger, err := cfg.Build()
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

// Logging is a simple mixin for types with attached loggers.
type Logging struct {
	logger  *zap.Logger
	sugared *zap.SugaredLogger
}

// NewLogging is a convenience method for constructing a Logging.
func NewLogging(logger *zap.Logger) Logging {
	return Logging{
		logger:  logger,
		sugared: logger.Sugar(),
	}
}

// L returns the raw logger.
func (l *Logging) L() *zap.Logger {
	return l.logger
}

// S returns the sugared logger.
func (l *Logging) S() *zap.SugaredLogger {
	return l.sugared
}
