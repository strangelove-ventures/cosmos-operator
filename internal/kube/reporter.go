package kube

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

// The two accepted event types for recording events.
const (
	EventWarning = "Warning"
	EventNormal  = "Normal"
)

// Logger is a structured logger
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
	Error(err error, msg string, keysAndValues ...interface{})
}

// Reporter logs and reports various events.
type Reporter interface {
	Logger
	RecordInfo(reason, msg string)
	RecordError(reason string, err error)
}

// ToLogger converts a logr.Logger to a Logger.
func ToLogger(log logr.Logger) Logger {
	return logger{log}
}

type logger struct {
	logr.Logger
}

func (l logger) Debug(msg string, keysAndValues ...interface{}) {
	l.V(1).Info(msg, keysAndValues...)
}

// EventReporter both logs and records events.
type EventReporter struct {
	log      Logger
	recorder record.EventRecorder
	resource runtime.Object
}

func NewEventReporter(logger logr.Logger, recorder record.EventRecorder, resource runtime.Object) EventReporter {
	return EventReporter{log: ToLogger(logger), recorder: recorder, resource: resource}
}

// Error logs as an error log entry.
func (r EventReporter) Error(err error, msg string, keysAndValues ...interface{}) {
	r.log.Error(err, msg, keysAndValues...)
}

// Info logs as an info log entry.
func (r EventReporter) Info(msg string, keysAndValues ...interface{}) {
	r.log.Info(msg, keysAndValues...)
}

// Debug logs as a debug log entry.
func (r EventReporter) Debug(msg string, keysAndValues ...interface{}) {
	r.log.Debug(msg, keysAndValues...)
}

// RecordError records a warning event.
func (r EventReporter) RecordError(reason string, err error) {
	r.recorder.Event(r.resource, EventWarning, reason, err.Error())
}

// RecordInfo records a normal event.
func (r EventReporter) RecordInfo(reason, msg string) {
	r.recorder.Event(r.resource, EventNormal, reason, msg)
}
