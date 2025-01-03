package inits

import (
	"fmt"
	"go.uber.org/zap"
)

func Logger(debugMode bool) (l *zap.Logger, err error) {
	if debugMode {
		l, err = zap.NewDevelopment()
	} else {
		l, err = zap.NewProduction()
	}
	if err != nil {
		return nil, fmt.Errorf("init logger: %w", err)
	}

	return l, nil
}
