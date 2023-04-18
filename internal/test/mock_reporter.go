package test

// NopReporter is a no-op kube.Reporter.
type NopReporter struct{}

func (n NopReporter) Info(msg string, keysAndValues ...interface{})             {}
func (n NopReporter) Debug(msg string, keysAndValues ...interface{})            {}
func (n NopReporter) Error(err error, msg string, keysAndValues ...interface{}) {}
func (n NopReporter) RecordInfo(reason, msg string)                             {}
func (n NopReporter) RecordError(reason string, err error)                      {}
