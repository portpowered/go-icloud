package icloud

import "github.com/portpowered/go-icloud/pkg/dependencymodels/httpboundary"

func nativeJSONMedia() string {
	return string(httpboundary.HTTPJSONMediaApplicationJSON)
}
