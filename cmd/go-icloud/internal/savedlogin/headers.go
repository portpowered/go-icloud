package savedlogin

import (
	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/referenceconfig"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

func referenceHeaders(china bool) []icloud.Header {
	origin, referer := referenceconfig.GlobalHome, referenceconfig.GlobalReferer
	if china {
		origin, referer = referenceconfig.ChinaHome, referenceconfig.ChinaReferer
	}

	return []icloud.Header{
		{Name: string(referenceconfig.UserAgentHeader), Value: string(referenceconfig.SafariUserAgent)},
		{Name: string(referenceconfig.AcceptHeader), Value: string(referenceconfig.AnyAccept)},
		{Name: string(referenceconfig.EncodingHeader), Value: string(referenceconfig.GzipDeflate)},
		{Name: string(referenceconfig.ConnectionHeader), Value: string(referenceconfig.KeepAlive)},
		{Name: string(referenceconfig.OriginHeader), Value: string(origin)},
		{Name: string(referenceconfig.RefererHeader), Value: string(referer)},
	}
}
