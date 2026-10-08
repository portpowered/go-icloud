package icloud

import "context"

// Lookup selects a child by its exact computed display name.
func (entry *DriveEntry) Lookup(ctx context.Context, request DriveLookupRequest) (*DriveEntry, error) {
	call, finish, err := entry.session.begin(ctx, "DriveLookup")
	if err != nil {
		return nil, err
	}
	defer finish()

	snapshot, err := entry.Snapshot()
	if err != nil {
		return nil, err
	}

	if snapshot.Type == "file" {
		return nil, newClientError("DriveLookup", NotDirectory, 0, nil, nil, errDriveDirectory)
	}

	result, err := entry.childrenInCall(call, false)
	if err != nil {
		return nil, err
	}

	for _, child := range result.Entries {
		data, snapshotErr := child.Snapshot()
		if snapshotErr != nil {
			return nil, snapshotErr
		}

		if data.Name == request.Name {
			return child, nil
		}
	}

	return nil, newClientError("DriveLookup", NotFound, 0, nil, nil, errDriveChild)
}

// Directory returns ordered child display names; a file produces a typed local refusal.
func (entry *DriveEntry) Directory(ctx context.Context, _ DriveEntryRequest) (*DriveDirectoryResult, error) {
	call, finish, err := entry.session.begin(ctx, "DriveDirectory")
	if err != nil {
		return nil, err
	}
	defer finish()

	snapshot, err := entry.Snapshot()
	if err != nil {
		return nil, err
	}

	if snapshot.Type == "file" {
		return nil, newClientError("DriveDirectory", NotDirectory, 0, nil, nil, errDriveDirectory)
	}

	children, err := entry.childrenInCall(call, false)
	if err != nil {
		return nil, err
	}

	result := &DriveDirectoryResult{Names: make([]string, 0, len(children.Entries))}

	for _, child := range children.Entries {
		data, snapshotErr := child.Snapshot()
		if snapshotErr != nil {
			return nil, snapshotErr
		}

		result.Names = append(result.Names, data.Name)
	}

	return result, nil
}
