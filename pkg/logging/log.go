package logging

import (
	"strconv"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger  *zap.Logger
	sugared *zap.SugaredLogger
	level   = zap.NewAtomicLevelAt(zapcore.WarnLevel)
)

func init() {
	DevelopmentMode()
}

// SetLevel adjusts the level of the loggers.
func SetLevel(l zapcore.Level) {
	level.SetLevel(l)
}

// ConsoleMode switches logging output to TTY mode.
func ConsoleMode() {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = level
	cfg.DisableCaller = true
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.EncoderConfig.EncodeTime = func() zapcore.TimeEncoder {
		// close over the start time to protect it.
		start := time.Now()
		return func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			elapsed := t.Sub(start)
			enc.AppendString(strconv.FormatFloat(elapsed.Seconds(), 'f', 5, 64) + "s")
		}
	}()

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
