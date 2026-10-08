package icloud

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
)

// DriveEntry is a node bound to one explicit Drive session.
// Mutations acknowledge requests without changing cached metadata or retrying writes.
type DriveEntry struct {
	session  *DriveSession
	data     DriveNode
	children []*DriveEntry
}

// Snapshot returns copied provider data and reference-compatible display properties.
// Snapshots remain readable after the owning session closes.
func (entry *DriveEntry) Snapshot() (*DriveEntrySnapshot, error) {
	entry.session.mu.Lock()
	data, err := copyDriveData(entry.data)
	entry.session.mu.Unlock()

	if err != nil {
		return nil, newClientError("DriveSnapshot", InvalidResponse, 0, nil, nil, err)
	}

	return driveSnapshot(data)
}

// MarshalJSON emits a copied snapshot rather than session credentials or cache internals.
func (entry *DriveEntry) MarshalJSON() ([]byte, error) {
	snapshot, err := entry.Snapshot()
	if err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("encode Drive entry snapshot: %w", err)
	}

	return encoded, nil
}

// Children returns cached entries, or merges a fresh provider read when forced.
func (entry *DriveEntry) Children(ctx context.Context, request DriveChildrenRequest) (*DriveChildrenResult, error) {
	call, finish, err := entry.session.begin(ctx, "DriveChildren")
	if err != nil {
		return nil, err
	}
	defer finish()

	return entry.childrenInCall(call, request.Force)
}

func (entry *DriveEntry) childrenInCall(ctx context.Context, force bool) (*DriveChildrenResult, error) {
	entry.session.mu.Lock()
	data := entry.data
	children := append([]*DriveEntry(nil), entry.children...)
	entry.session.mu.Unlock()

	if len(children) != 0 && !force {
		return &DriveChildrenResult{Entries: children}, nil
	}

	if data.Items == nil || force {
		fresh, err := entry.fetch(ctx, data)
		if err != nil {
			return nil, err
		}

		data, err = mergeDriveData(data, fresh)
		if err != nil {
			return nil, newClientError("DriveChildren", InvalidResponse, 0, nil, nil, err)
		}

		entry.session.mu.Lock()
		entry.data = data
		entry.session.mu.Unlock()
	}

	if data.Items == nil {
		return nil, newClientError("DriveChildren", InvalidResponse, 0, nil, nil, errDriveItems)
	}

	children = make([]*DriveEntry, 0, len(*data.Items))
	for _, item := range *data.Items {
		children = append(children, &DriveEntry{session: entry.session, data: item, children: nil})
	}

	entry.session.mu.Lock()
	entry.children = children
	entry.session.mu.Unlock()

	return &DriveChildrenResult{Entries: append([]*DriveEntry{}, children...)}, nil
}

func (entry *DriveEntry) fetch(ctx context.Context, data DriveNode) (DriveNode, error) {
	var empty DriveNode
	if data.Drivewsid == nil {
		return empty, newClientError("DriveChildren", InvalidResponse, 0, nil, nil, errDriveFields)
	}

	result, err := entry.session.client.GetDriveNode(ctx, GetDriveNodeRequest{
		Auth: entry.session.Authentication(), NodeID: *data.Drivewsid, ShareID: data.ShareID})
	if err != nil {
		entry.session.observeFailure(err)

		return empty, err
	}

	entry.session.observe(result.Metadata)

	return result.Node, nil
}

func mergeDriveData(current, fresh DriveNode) (DriveNode, error) {
	var result DriveNode

	oldFields, err := driveDataFields(current)
	if err != nil {
		return result, fmt.Errorf("encode merged Drive node: %w", err)
	}

	newFields, err := driveDataFields(fresh)
	if err != nil {
		return result, err
	}

	maps.Copy(oldFields, newFields)

	encoded, err := json.Marshal(oldFields)
	if err != nil {
		return result, fmt.Errorf("encode merged Drive data: %w", err)
	}

	err = json.Unmarshal(encoded, &result)
	if err != nil {
		return result, fmt.Errorf("decode merged Drive node: %w", err)
	}

	return result, nil
}

func driveDataFields(data DriveNode) (map[string]json.RawMessage, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encode Drive fields: %w", err)
	}

	var fields map[string]json.RawMessage

	err = json.Unmarshal(encoded, &fields)
	if err != nil {
		return nil, fmt.Errorf("decode Drive fields: %w", err)
	}

	return fields, nil
}
