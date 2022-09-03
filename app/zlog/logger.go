package zlog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger type definition of looger
type Logger *zap.SugaredLogger

var _log Logger

// Debug uses fmt.Sprint to construct and log a message.
func Debug(args ...interface{}) {
	Get().Debug(args...)
}

// Info uses fmt.Sprint to construct and log a message.
func Info(args ...interface{}) {
	Get().Info(args...)
}

// Warn uses fmt.Sprint to construct and log a message.
func Warn(args ...interface{}) {
	Get().Warn(args...)
}

// Error uses fmt.Sprint to construct and log a message.
func Error(args ...interface{}) {
	Get().Error(args...)
}

// Panic uses fmt.Sprint to construct and log a message, then panics.
func Panic(args ...interface{}) {
	Get().Panic(args...)
}

// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
func Fatal(args ...interface{}) {
	Get().Fatal(args...)
}

// Debugf uses fmt.Sprintf to log a templated message.
func Debugf(template string, args ...interface{}) {
	Get().Debugf(template, args...)
}

// Infof uses fmt.Sprintf to log a templated message.
func Infof(template string, args ...interface{}) {
	Get().Infof(template, args...)
}

// Warnf uses fmt.Sprintf to log a templated message.
func Warnf(template string, args ...interface{}) {
	Get().Warnf(template, args...)
}

// Errorf uses fmt.Sprintf to log a templated message.
func Errorf(template string, args ...interface{}) {
	Get().Errorf(template, args...)
}

// Panicf uses fmt.Sprintf to log a templated message, then panics.
func Panicf(template string, args ...interface{}) {
	Get().Panicf(template, args...)
}

// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func Fatalf(template string, args ...interface{}) {
	Get().Fatalf(template, args...)
}

// Debugw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
//
// When debug-level logging is disabled, this is much faster than
//
//	s.With(keysAndValues).Debug(msg)
func Debugw(msg string, keysAndValues ...interface{}) {
	Get().Debugw(msg, keysAndValues...)
}

// Infow logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Infow(msg string, keysAndValues ...interface{}) {
	Get().Infow(msg, keysAndValues...)
}

// Warnw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Warnw(msg string, keysAndValues ...interface{}) {
	Get().Warnw(msg, keysAndValues...)
}

// Errorw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Errorw(msg string, keysAndValues ...interface{}) {
	Get().Errorw(msg, keysAndValues...)
}

// Panicw logs a message with some additional context, then panics. The
// variadic key-value pairs are treated as they are in With.
func Panicw(msg string, keysAndValues ...interface{}) {
	Get().Panicw(msg, keysAndValues...)
}

// Fatalw logs a message with some additional context, then calls os.Exit. The
// variadic key-value pairs are treated as they are in With.
func Fatalw(msg string, keysAndValues ...interface{}) {
	Get().Fatalw(msg, keysAndValues...)
}

// Get returns main logger
func Get() *zap.SugaredLogger {
	if _log == nil {
		panic("logger is absent")
	}
	return _log
}

// NewZapLogger creates new instance of zap logger
func NewZapLogger(verbose bool) Logger {
	var cfg zap.Config
	var stacktraceLevel zapcore.Level
	if verbose {
		cfg = zap.NewDevelopmentConfig()
		stacktraceLevel = zap.ErrorLevel
	} else {
		cfg = zap.NewProductionConfig()
		cfg.Encoding = "console"
		cfg.Sampling = nil
		stacktraceLevel = zap.FatalLevel + 1
	}
	cfg.EncoderConfig.ConsoleSeparator = " "
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zlogger, _ := cfg.Build(
		zap.WithCaller(verbose),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(stacktraceLevel),
	)
	_log = zlogger.Sugar()
	return _log
}

// Sync flushes the log buffer
func Sync() error {
	return Get().Sync()
}
