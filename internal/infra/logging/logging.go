// Package logging provides context-based logging utilities for the toolkit application.
package logging

import (
	"go.uber.org/zap"
)

// NewLogger creates a new Logger. If debug is true, uses zap.NewDevelopment, else zap.NewProduction.
func NewLogger(debug bool) (Logger, error) {
	var zl *zap.Logger
	var err error
	if debug {
		zl, err = zap.NewDevelopment()
	} else {
		zl, err = zap.NewProduction()
	}
	if err != nil {
		return nil, err
	}
	return NewZapLogger(zl.Sugar()), nil
}
