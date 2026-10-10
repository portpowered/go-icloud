package webtransport

import (
	"net/http"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/httpboundary"
)

// CallerHeaders copies explicitly caller-owned extension names and values.
// It preserves the public HTTP-header input used by the transport boundary.
func CallerHeaders(headers http.Header) http.Header {
	if headers == nil {
		return http.Header(make(httpboundary.CallerHeaderMap))
	}

	return http.Header(httpboundary.CallerHeaderMap(headers)).Clone()
}
