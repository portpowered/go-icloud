package icloud

import "context"

// Download uses the entry's document ID and zone, with the reference numeric-zero shortcut.
// Returned bytes belong to the caller; a local empty result has no HTTP metadata.
func (entry *DriveEntry) Download(ctx context.Context, _ DriveEntryRequest) (*DriveEntryDownloadResult, error) {
	call, finish, err := entry.session.begin(ctx, "DriveEntryDownload")
	if err != nil {
		return nil, err
	}
	defer finish()

	snapshot, err := entry.Snapshot()
	if err != nil {
		return nil, err
	}

	if snapshot.Data.Size != nil && string(*snapshot.Data.Size) == "0" {
		return &DriveEntryDownloadResult{Content: []byte{}, LocalEmpty: true, Metadata: nil, TokenMetadata: nil}, nil
	}

	if snapshot.Data.Docwsid == nil || snapshot.Data.Zone == nil {
		return nil, newClientError("DriveEntryDownload", InvalidResponse, 0, nil, nil, errDriveFields)
	}

	result, err := entry.session.client.DownloadDriveFile(call, DownloadDriveFileRequest{
		Auth: entry.session.Authentication(), DocumentID: *snapshot.Data.Docwsid, Zone: snapshot.Data.Zone})
	if err != nil {
		entry.session.observeFailure(err)

		return nil, err
	}

	entry.session.observe(result.TokenMetadata, result.Metadata)

	return &DriveEntryDownloadResult{Content: result.Content, LocalEmpty: false,
		Metadata: &result.Metadata, TokenMetadata: &result.TokenMetadata}, nil
}

// Upload binds preparation to this entry's document ID/zone and keeps the caller's reader open.
func (entry *DriveEntry) Upload(ctx context.Context, request DriveUploadRequest) (*UploadDriveFileResult, error) {
	call, finish, err := entry.session.begin(ctx, "DriveEntryUpload")
	if err != nil {
		return nil, err
	}
	defer finish()

	snapshot, err := entry.Snapshot()
	if err != nil {
		return nil, err
	}

	if snapshot.Data.Docwsid == nil || snapshot.Data.Zone == nil {
		return nil, newClientError("DriveEntryUpload", InvalidResponse, 0, nil, nil, errDriveFields)
	}

	result, err := entry.session.client.UploadDriveFile(call, UploadDriveFileRequest{
		Auth: entry.session.Authentication(), ParentID: *snapshot.Data.Docwsid, Zone: snapshot.Data.Zone,
		Filename: request.Filename, Content: request.Content, CreationTime: request.CreationTime,
		ModificationTime: request.ModificationTime})
	if err != nil {
		entry.session.observeFailure(err)

		return nil, err
	}

	entry.session.setToken(result.UploadToken)
	entry.session.observe(result.PreparationMetadata, result.TransferMetadata, result.RegistrationMetadata)

	return result, nil
}
