package webtransport

import (
	"fmt"
	"net/url"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/httpboundary"
)

// contentLocatorTarget binds the compatibility argument to its generated boundary record.
// Each operation validates the returned target before constructing or sending a request.
func contentLocatorTarget(contentURL string) (*url.URL, error) {
	locator := httpboundary.ContentLocator{Url: contentURL}

	target, err := url.Parse(locator.Url)
	if err != nil {
		return nil, fmt.Errorf("parse content locator: %w", err)
	}

	return target, nil
}
