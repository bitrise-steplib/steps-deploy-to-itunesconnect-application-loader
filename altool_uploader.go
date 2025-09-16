package main

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/bitrise-io/go-utils/v2/log"
)

//	"user-info" : {
//		"NSLocalizedDescription" : "The provided entity includes an attribute with a value that has already been used",
//		"NSLocalizedFailureReason" : "The bundle version must be higher than the previously uploaded version.",
//		"code" : "ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE",
//		"detail" : "The bundle version must be higher than the previously uploaded version.",
//		"id" : "c1a5ca15-d0c8-49b4-893f-dc187711f6f0",
//		"meta" : "{\n    previousBundleVersion = 2509121012571647;\n}",
//		"source" : "{\n    pointer = \"/data/attributes/cfBundleVersion\";\n}",
//		"status" : "409",
//		"title" : "The provided entity includes an attribute with a value that has already been used"
//	}
type userInfo struct {
	NSLocalizedDescription   string `json:"NSLocalizedDescription"`
	NSLocalizedFailureReason string `json:"NSLocalizedFailureReason"`
	// Lower level only, optional:
	Code   string `json:"code"` // same as IrisCode e.g. "STATE_ERROR.VALIDATION_ERROR"
	Detail string `json:"detail"`
	ID     string `json:"id"`
	Meta   string `json:"meta"`
	Source string `json:"source"`
	Status string `json:"status"`
	Title  string `json:"title"`
	// Top level only, optional:
	NSUnderlyingError string `json:"NSUnderlyingError"`
	IrisCode          string `json:"iris-code"`
}

type productError struct {
	Code             int            `json:"code"`
	Message          string         `json:"message"`
	UserInfo         userInfo       `json:"user-info"`
	UnderlyingErrors []productError `json:"underlying-errors"`
}

//	"details" : {
//	  "delivery-uuid" : "2d29ae8f-a628-4fee-bb75-d0fa4331d23c",
//	  "transferred" : "19555969 bytes in 1.831 seconds (10.7MB/s, 85.437Mbps)"
//	},
type successDetails struct {
	DeliveryUUID string `json:"delivery-uuid"`
	Transferred  string `json:"transferred"`
}

type altoolResult struct {
	SuccessMessage string         `json:"success-message"`
	FailureMessage string         `json:"-"`
	SuccessDetails successDetails `json:"details"`

	ProductErrors []productError `json:"product-errors"`
	Warnings      []productError `json:"warnings"`

	OSVersion   string `json:"os-version"`
	ToolVersion string `json:"tool-version"`
	ToolPath    string `json:"tool-path"`
}

func (a altoolResult) getError() error {
	if a.SuccessMessage != "" {
		return nil
	}

	numErrors := len(a.ProductErrors)
	if numErrors == 0 {
		return fmt.Errorf("upload failed, but no error message found")
	}

	firstErr := newUploadErrorFromProductError(a.ProductErrors[0])
	if numErrors == 1 {
		return fmt.Errorf("%w", firstErr)
	}
	return fmt.Errorf("%d errors, first: %w", numErrors, firstErr)
}

func (a altoolResult) getWarnings() []error {
	var warnings []error
	for _, w := range a.Warnings {
		warnings = append(warnings, newUploadErrorFromProductError(w))
	}

	return warnings
}

func parseJSONOutput(_ log.Logger, stdOut string) (altoolResult, error) {
	jsonRegexp := regexp.MustCompile(`(?m)^\s*{\n?(.*\n?)*}\s*$`)
	match := jsonRegexp.FindString(stdOut)
	if match == "" {
		return altoolResult{}, fmt.Errorf("failed to find JSON output in altool output: %s", stdOut)
	}

	var output altoolResult
	if err := json.Unmarshal([]byte(match), &output); err != nil {
		return altoolResult{}, fmt.Errorf("failed to parse altool JSON output %w, out: %s", err, stdOut)
	}

	return output, nil
}

func parseAltoolOutput(logger log.Logger, stdOut, errorOut string, isJson bool) (altoolResult, error) {
	var err error
	if isJson {
		var result altoolResult
		if result, err = parseJSONOutput(logger, stdOut); err == nil {
			return result, result.getError()
		}
		logger.Warnf("Could not parse altool output as JSON: %v, out: %s", err, stdOut)
	}

	// Fallback to text parsing
	errorRe := regexp.MustCompile(`(?s).*ERROR:.*`)
	sucessRe := regexp.MustCompile(`(?s).*UPLOAD SUCCEEDED.*`)
	if errorRe.MatchString(errorOut) && !sucessRe.MatchString(stdOut) && !sucessRe.MatchString(errorOut) {
		return altoolResult{}, fmt.Errorf("%s", errorOut)
	}

	return altoolResult{SuccessMessage: "Upload succeeded"}, nil
}
