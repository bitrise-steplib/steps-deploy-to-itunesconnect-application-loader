package main

import "fmt"

type uploadError struct {
	description string
	reason      string
	errorCode   int
	errorID     string
}

func newUploadErrorFromProductError(pe productError) uploadError {
	return uploadError{
		description: pe.UserInfo.NSLocalizedDescription,
		reason:      pe.UserInfo.NSLocalizedFailureReason,
		errorCode:   pe.Code,
		errorID:     pe.UserInfo.IrisCode,
	}
}

func (e uploadError) Error() string {
	msg := e.description
	if e.errorCode != 0 {
		msg += fmt.Sprintf(" (%d)", e.errorCode)
	}
	if e.reason != "" {
		msg += fmt.Sprintf("  %s", e.reason)
	}
	if e.errorID != "" {
		msg += fmt.Sprintf("  (code: %s)", e.errorID)
	}
	return msg
}
