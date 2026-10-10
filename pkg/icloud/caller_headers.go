package icloud

import (
	"net/http"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/httpboundary"
)

func requestHeaders(headers []Header) http.Header {
	owned := make(httpboundary.CallerHeaderMap)
	result := http.Header(owned)

	for _, header := range headers {
		record := httpboundary.CallerHeader{Name: header.Name,
			Values: httpboundary.CallerHeaderValues{header.Value}}
		for _, value := range record.Values {
			result.Add(record.Name, value)
		}
	}

	return result
}
