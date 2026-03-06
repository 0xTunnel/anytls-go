package main

import (
	"anytls/util"

	"github.com/sirupsen/logrus"
)

func eventLogger(component string, fields logrus.Fields, event string) *logrus.Entry {
	entryFields := logrus.Fields{
		"component": component,
		"event":     event,
	}
	for key, value := range fields {
		entryFields[key] = value
	}
	return logrus.WithFields(entryFields)
}

func diagnosticLoggingEnabled() bool {
	return util.DiagnosticLoggingEnabled()
}

func logDiagnostic(component string, fields logrus.Fields, event string, message string) {
	if !diagnosticLoggingEnabled() {
		return
	}
	eventLogger(component, fields, event).Debug(message)
}
