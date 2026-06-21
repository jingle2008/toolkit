package logging

import "errors"

// tee forwards every log call to all wrapped loggers.
type tee struct {
	loggers []Logger
}

// NewTee returns a Logger that fans every call out to all loggers.
func NewTee(loggers ...Logger) Logger {
	return &tee{loggers: loggers}
}

func (t *tee) Debugw(msg string, kv ...any) {
	for _, l := range t.loggers {
		l.Debugw(msg, kv...)
	}
}

func (t *tee) Infow(msg string, kv ...any) {
	for _, l := range t.loggers {
		l.Infow(msg, kv...)
	}
}

func (t *tee) Warnw(msg string, kv ...any) {
	for _, l := range t.loggers {
		l.Warnw(msg, kv...)
	}
}

func (t *tee) Errorw(msg string, kv ...any) {
	for _, l := range t.loggers {
		l.Errorw(msg, kv...)
	}
}

func (t *tee) WithFields(kv ...any) Logger {
	next := make([]Logger, len(t.loggers))
	for i, l := range t.loggers {
		next[i] = l.WithFields(kv...)
	}
	return &tee{loggers: next}
}

func (t *tee) DebugEnabled() bool {
	for _, l := range t.loggers {
		if l.DebugEnabled() {
			return true
		}
	}
	return false
}

func (t *tee) Sync() error {
	var errs []error
	for _, l := range t.loggers {
		if err := l.Sync(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
