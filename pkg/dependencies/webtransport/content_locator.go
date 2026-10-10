package webtransport

import (
	"net/url"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/httpboundary"
)

// contentLocatorTarget binds the compatibility argument to its generated boundary record.
// Each operation validates the returned target before constructing or sending a request.
func contentLocatorTarget(contentURL string) (*url.URL, error) {
	locator := httpboundary.ContentLocator{Url: contentURL}

	return url.Parse(locator.Url)
}
