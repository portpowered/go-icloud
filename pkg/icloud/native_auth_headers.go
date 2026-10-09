package icloud

import (
	"net/http"

	"github.com/portpowered/go-icloud/internal/protocol"
)

func nativeAuthHeaders(state NativeAuthState, accept string) http.Header {
	headers := requestHeaders(state.Auth.Headers)
	headers.Set(protocol.AuthHTTPAcceptName, accept)
	headers.Set(protocol.AuthHTTPContentTypeName, protocol.AuthMediaApplicationJson)
	headers.Set(protocol.AuthHTTPXAppleOAuthClientIdName, protocol.AuthOAuthClientIDValue)
	headers.Set(protocol.AuthHTTPXAppleOAuthClientTypeName, protocol.AuthOAuthClientTypeValue)
	headers.Set(protocol.AuthHTTPXAppleOAuthRedirectURIName, nativeHomeOrigin(state))
	headers.Set(protocol.AuthHTTPXAppleOAuthRequireGrantCodeName, protocol.AuthOAuthRequireGrantCodeValue)
	headers.Set(protocol.AuthHTTPXAppleOAuthResponseModeName, protocol.AuthOAuthResponseModeValue)
	headers.Set(protocol.AuthHTTPXAppleOAuthResponseTypeName, protocol.AuthOAuthResponseTypeValue)
	headers.Set(protocol.AuthHTTPXAppleOAuthStateName, state.Auth.ClientID)
	headers.Set(protocol.AuthHTTPXAppleWidgetKeyName, protocol.AuthOAuthClientIDValue)
	headers.Set(protocol.AuthHTTPXAppleFDClientInfoName, protocol.AuthFDClientInfoValue)
	headers.Set(protocol.AuthHTTPRefererName, nativeIDMSOrigin(state))
	headers.Set(protocol.AuthHTTPXAppleFrameIdName, state.Auth.ClientID)

	prior := requestHeaders(state.Auth.Headers)
	for _, name := range []string{protocol.AuthHTTPScntName, protocol.AuthHTTPXAppleIDSessionIdName,
		protocol.AuthHTTPXAppleAuthAttributesName} {
		if value := prior.Get(name); value != "" {
			headers.Set(name, value)
		}
	}

	return headers
}
