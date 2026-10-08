package icloud

import "context"

type driveEntryAction uint8

const (
	driveEntryRename driveEntryAction = iota
	driveEntryTrash
	driveEntryDelete
	driveEntryRestore
	driveEntryDeleteForever
)

// CreateFolder creates a child folder under this entry's provider Drive identifier.
func (entry *DriveEntry) CreateFolder(ctx context.Context,
	request DriveFolderRequest,
) (*CreateDriveFolderResult, error) {
	call, finish, err := entry.session.begin(ctx, "DriveEntryCreateFolder")
	if err != nil {
		return nil, err
	}
	defer finish()

	snapshot, err := entry.Snapshot()
	if err != nil {
		return nil, err
	}

	if snapshot.Data.Drivewsid == nil {
		return nil, newClientError("DriveEntryCreateFolder", InvalidResponse, 0, nil, nil, errDriveFields)
	}

	result, err := entry.session.client.CreateDriveFolder(call, CreateDriveFolderRequest{
		Auth: entry.session.Authentication(), ParentID: *snapshot.Data.Drivewsid, Name: request.Name})
	if err != nil {
		entry.session.observeFailure(err)

		return nil, err
	}

	entry.session.observe(result.Metadata)

	return result, nil
}

// Rename acknowledges a rename without rewriting the entry's cached name/version.
func (entry *DriveEntry) Rename(ctx context.Context, request DriveRenameRequest) (*RenameDriveNodeResult, error) {
	return entry.mutate(ctx, driveEntryRename, request.Name)
}

// Trash requests movement into the provider trash without retrying the write.
func (entry *DriveEntry) Trash(ctx context.Context, _ DriveEntryRequest) (*TrashDriveNodeResult, error) {
	return entry.mutate(ctx, driveEntryTrash, "")
}

// Delete requests ordinary deletion without retrying the write.
func (entry *DriveEntry) Delete(ctx context.Context, _ DriveEntryRequest) (*DeleteDriveNodeResult, error) {
	return entry.mutate(ctx, driveEntryDelete, "")
}

// Restore refuses entries without a restore path before issuing a recovery request.
func (entry *DriveEntry) Restore(ctx context.Context, _ DriveEntryRequest) (*RestoreDriveNodeResult, error) {
	return entry.mutate(ctx, driveEntryRestore, "")
}

// PermanentlyDelete refuses entries outside trash before issuing an irreversible request.
func (entry *DriveEntry) PermanentlyDelete(ctx context.Context,
	_ DriveEntryRequest,
) (*PermanentlyDeleteDriveNodeResult, error) {
	return entry.mutate(ctx, driveEntryDeleteForever, "")
}

func (entry *DriveEntry) mutate(ctx context.Context,
	action driveEntryAction, name string,
) (*DriveItemChangeResult, error) {
	call, finish, err := entry.session.begin(ctx, "DriveEntryMutation")
	if err != nil {
		return nil, err
	}
	defer finish()

	snapshot, err := entry.Snapshot()
	if err != nil {
		return nil, err
	}

	data := snapshot.Data
	if (action == driveEntryRestore || action == driveEntryDeleteForever) && driveString(data.RestorePath) == "" {
		return nil, newClientError("DriveEntryMutation", NotInTrash, 0, nil, nil, errDriveTrash)
	}

	if data.Drivewsid == nil || data.Etag == nil {
		return nil, newClientError("DriveEntryMutation", InvalidResponse, 0, nil, nil, errDriveFields)
	}

	result, err := entry.callMutation(call, action, name, DriveNodeSelector{NodeID: *data.Drivewsid, ETag: *data.Etag})
	if err != nil {
		entry.session.observeFailure(err)

		return nil, err
	}

	entry.session.observe(result.Metadata)

	return result, nil
}

func (entry *DriveEntry) callMutation(ctx context.Context, action driveEntryAction,
	name string, node DriveNodeSelector,
) (*DriveItemChangeResult, error) {
	auth := entry.session.Authentication()
	client := entry.session.client

	switch action {
	case driveEntryRename:
		return client.RenameDriveNode(ctx, RenameDriveNodeRequest{Auth: auth, Node: node, Name: name})
	case driveEntryTrash:
		return client.TrashDriveNode(ctx, TrashDriveNodeRequest{Auth: auth, Node: node})
	case driveEntryDelete:
		return client.DeleteDriveNode(ctx, DeleteDriveNodeRequest{Auth: auth, Node: node})
	case driveEntryRestore:
		return client.RestoreDriveNode(ctx, RestoreDriveNodeRequest{Auth: auth, Node: node})
	case driveEntryDeleteForever:
		return client.PermanentlyDeleteDriveNode(ctx, PermanentlyDeleteDriveNodeRequest{Auth: auth, Node: node})
	default:
		return nil, newClientError("DriveEntryMutation", Configuration, 0, nil, nil, errDriveFields)
	}
}
