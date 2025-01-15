package log

import "go.uber.org/zap"

var l *zap.SugaredLogger

func InitLogger() {

	level := zap.NewAtomicLevelAt(zap.DebugLevel)
	logger, err := zap.Config{
		Level:            level,
		Development:      true,
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}.Build()

	if err != nil {
		panic(err)
	}
	l = logger.Sugar()
}

func Infoln(msg string, args ...any) {
	l.Infoln(msg, args)
}

func Errorln(msg string, args ...any) {
	l.Errorln(msg, args)
}

func Debugln(msg string, args ...any) {
	l.Debugln(msg, args)
}

func Warnln(msg string, args ...any) {
	l.Warnln(msg, args)
}
