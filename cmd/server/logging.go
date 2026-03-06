package main

import "github.com/sirupsen/logrus"

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
