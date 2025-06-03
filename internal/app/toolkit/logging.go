package toolkit

import (
	"sync"

	"go.uber.org/zap"
)

var (
	logger     *zap.Logger
	loggerOnce sync.Once
)

// Logger returns a singleton zap.Logger for the toolkit package.
func Logger() *zap.Logger {
	loggerOnce.Do(func() {
		l, err := zap.NewDevelopment()
		if err != nil {
			panic(err)
		}
		logger = l
	})
	return logger
}
