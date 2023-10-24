package cmd

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func ZapLogger(level, format string) *zap.Logger {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format(time.RFC3339))
	}
	enc := zapcore.NewConsoleEncoder(config)
	if format == "json" {
		enc = zapcore.NewJSONEncoder(config)
	}

	lvl := zap.NewAtomicLevel()
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	return zap.New(zapcore.NewCore(enc, os.Stdout, lvl))
}
