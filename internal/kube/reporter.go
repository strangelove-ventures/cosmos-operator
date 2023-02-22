package kube

import (
	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Reporter logs and reports various events.
type Reporter interface {
	Info(msg string, keysAndValues ...interface{})
	Error(err error, msg string, keysAndValues ...interface{})
	RecordInfo(reason, msg string)
	RecordError(reason string, err error)
}

// EventReporter both logs and records events.
type EventReporter struct {
	log      logr.Logger
	recorder record.EventRecorder
	resource client.Object
}

func NewEventReporter(recorder record.EventRecorder) EventReporter {
	return EventReporter{log: logr.Discard(), recorder: recorder}
}

func (r EventReporter) WithResource(resource client.Object) EventReporter {
	r.resource = resource
	return r
}

func (r EventReporter) WithLogger(logger logr.Logger) EventReporter {
	r.log = logger
	return r
}

// Error logs as an error log entry.
func (r EventReporter) Error(err error, msg string, keysAndValues ...interface{}) {
	r.log.Error(err, msg, keysAndValues...)
}

// Info logs as an info log entry.
func (r EventReporter) Info(msg string, keysAndValues ...interface{}) {
	r.log.Info(msg, keysAndValues...)
}

// RecordError records a warning event.
func (r EventReporter) RecordError(reason string, err error) {
	const label = "Warning"
	r.recorder.Event(r.resource, label, reason, err.Error())
}

// RecordInfo records a normal event.
func (r EventReporter) RecordInfo(reason, msg string) {
	const label = "Normal"
	r.recorder.Event(r.resource, label, reason, msg)
}
