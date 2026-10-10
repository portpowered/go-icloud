package webtransport

import "github.com/portpowered/go-icloud/pkg/dependencymodels/httpboundary"

func jsonMedia() string {
	return string(httpboundary.HTTPJSONMediaApplicationJSON)
}

func plainTextMedia() string {
	return string(httpboundary.HTTPPlainTextMediaPlainText)
}

func anyMedia() string {
	return string(httpboundary.HTTPAnyMediaWildcard)
}
