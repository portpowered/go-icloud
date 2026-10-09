package icloud

import "context"

// GetLegacyRemindersSnapshot reads startup lists and reminders from the discovered legacy service.
// Records retain their provider fields. This is not a CloudKit or completed-item snapshot.
func (sdk *SDK) GetLegacyRemindersSnapshot(ctx context.Context,
	request GetLegacyRemindersSnapshotRequest,
) (*GetLegacyRemindersSnapshotResult, error) {
	const operation = "GetLegacyRemindersSnapshot"

	boundary, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	boundary.Origin = request.Auth.LegacyRemindersServiceURL

	response, err := sdk.web.LegacyRemindersStartup(ctx, boundary)
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return &GetLegacyRemindersSnapshotResult{Lists: response.Data.Collections, Reminders: response.Data.Reminders,
		Metadata: publicMetadata(response.Metadata)}, nil
}
