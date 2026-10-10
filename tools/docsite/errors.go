package main

import "errors"

var (
	errEmptyExpectations  = errors.New("content expectations are empty")
	errExpectedPage       = errors.New("expected rendered page missing")
	errExpectedContent    = errors.New("expected rendered content absent")
	errMissingFragment    = errors.New("missing fragment")
	errProjectLink        = errors.New("link leaves project site")
	errMissingDestination = errors.New("missing local destination")
	errEmptySite          = errors.New("site has no rendered HTML pages")
)
