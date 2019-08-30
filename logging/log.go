package logging

import "go.uber.org/zap"

var (
	logger  *zap.Logger
	sugared *zap.SugaredLogger
)

func init() {
	var err error
	logger, err = zap.NewDevelopment()
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
