package icloud

import "github.com/portpowered/go-icloud/pkg/dependencies/webtransport/authapi"

func nativeAuthParams(state NativeAuthState) authapi.ListAuthTrustedDevicesParams {
	params := authapi.ListAuthTrustedDevicesParams{ClientBuildNumber: state.Auth.ClientBuildNumber,
		ClientMasteringNumber: state.Auth.ClientMasteringNumber, ClientId: &state.Auth.ClientID, Dsid: nil, Cookie: nil}
	if state.Auth.AccountID != "" {
		params.Dsid = &state.Auth.AccountID
	}

	return params
}
