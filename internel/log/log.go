package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

var (
	Log *zap.SugaredLogger
)

func InitLogger() {
	Log = newLogger().Sugar()
}

func newLogger() *zap.Logger {
	encoder := getEncoder()
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		zap.DebugLevel,
	)
	return zap.New(core, zap.AddCaller())
}

func getEncoder() zapcore.Encoder {
	conf := zap.NewProductionEncoderConfig()
	conf.TimeKey = "time"
	conf.EncodeTime = zapcore.ISO8601TimeEncoder
	return zapcore.NewConsoleEncoder(conf)
}
