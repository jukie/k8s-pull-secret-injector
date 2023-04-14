// logger/logger.go
package logger

import (
	"go.uber.org/zap"
)

var Log *zap.Logger

func InitLogger() error {
	var err error
	Log, err = zap.NewProduction()
	if err != nil {
		return err
	}
	return nil
}

func Sync() {
	_ = Log.Sync()
}
