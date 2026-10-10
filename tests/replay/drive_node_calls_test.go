package replay_test

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func (runner *driveNodeRunner) call(t *testing.T, call driveNodeCall) (any, error) {
	t.Helper()

	switch call.Operation {
	case nodeRootLabel, nodeTrashLabel, nodeRefreshRoot, nodeRefreshTrash:
		return runner.location(t, call.Operation)
	case "children":
		force := false
		driveNodeKeyword(t, call, "force", &force)

		result, err := runner.node.Children(t.Context(), icloud.DriveChildrenRequest{Force: force})
		if err != nil {
			return nil, fmt.Errorf(replayNodeErrorFormat, err)
		}

		return portableDriveChildren(t, result.Entries), nil
	case "get", "getitem", replayLiteralServiceGetitem:
		return runner.lookup(t, call)
	case "metadata":
		return portableDriveSnapshot(t, runner.node), nil
	default:
		return runner.endpoint(t, call)
	}
}

func (runner *driveNodeRunner) location(t *testing.T, operation string) (any, error) {
	t.Helper()

	var (
		entry *icloud.DriveEntry
		err   error
	)

	switch operation {
	case nodeRootLabel, nodeRefreshRoot:
		entry, err = runner.session.Root(t.Context(), icloud.DriveLocationRequest{Refresh: operation == nodeRefreshRoot})
	case nodeTrashLabel, nodeRefreshTrash:
		entry, err = runner.session.Trash(t.Context(), icloud.DriveLocationRequest{Refresh: operation == nodeRefreshTrash})
	default:
		t.Fatal("unknown location")
	}

	if err != nil {
		return nil, fmt.Errorf(replayNodeErrorFormat, err)
	}

	runner.node = entry

	return portableDriveSnapshot(t, entry), nil
}

func (runner *driveNodeRunner) lookup(t *testing.T, call driveNodeCall) (any, error) {
	t.Helper()
	request := icloud.DriveLookupRequest{Name: driveStringInput(t, call.Inputs, 0)}

	var (
		node *icloud.DriveEntry
		err  error
	)

	if call.Operation == replayLiteralServiceGetitem {
		node, err = runner.session.Lookup(t.Context(), request)
	} else {
		node, err = runner.node.Lookup(t.Context(), request)
	}

	if err != nil {
		return nil, fmt.Errorf(replayNodeErrorFormat, err)
	}

	runner.node = node

	return portableDriveSnapshot(t, node), nil
}

func (runner *driveNodeRunner) endpoint(t *testing.T, call driveNodeCall) (any, error) {
	t.Helper()

	switch call.Operation {
	case "dir", replayLiteralServiceDir:
		return runner.directory(t, call.Operation)
	case "open":
		value, err := runner.node.Download(t.Context(), icloud.DriveEntryRequest{})
		if err != nil {
			return nil, fmt.Errorf(replayNodeErrorFormat, err)
		}

		return portableDriveDownloaded(t, value), nil
	case "upload":
		return runner.upload(t, call)
	case "mkdir":
		value, err := runner.node.CreateFolder(t.Context(), icloud.DriveFolderRequest{
			Name: driveStringInput(t, call.Inputs, 0)})
		if err != nil {
			return nil, fmt.Errorf(replayNodeErrorFormat, err)
		}

		fields := make(map[string]any, len(value.AdditionalMetadata)+1)
		for name, raw := range value.AdditionalMetadata {
			fields[name] = raw
		}

		if value.Folders != nil {
			fields["folders"] = value.Folders
		}

		return fields, nil
	default:
		return runner.mutate(t, call)
	}
}

func (runner *driveNodeRunner) directory(t *testing.T, operation string) (any, error) {
	t.Helper()

	var (
		value *icloud.DriveDirectoryResult
		err   error
	)

	if operation == replayLiteralServiceDir {
		value, err = runner.session.Directory(t.Context(), icloud.DriveEntryRequest{})
	} else {
		value, err = runner.node.Directory(t.Context(), icloud.DriveEntryRequest{})
	}

	if err != nil {
		return nil, fmt.Errorf(replayNodeErrorFormat, err)
	}

	return value.Names, nil
}

func (runner *driveNodeRunner) mutate(t *testing.T, call driveNodeCall) (any, error) {
	t.Helper()

	var (
		value *icloud.DriveItemChangeResult
		err   error
	)

	switch call.Operation {
	case "rename":
		value, err = runner.node.Rename(t.Context(), icloud.DriveRenameRequest{Name: driveStringInput(t, call.Inputs, 0)})
	case "move_to_trash":
		value, err = runner.node.Trash(t.Context(), icloud.DriveEntryRequest{})
	case "delete":
		value, err = runner.node.Delete(t.Context(), icloud.DriveEntryRequest{})
	case "recover":
		value, err = runner.node.Restore(t.Context(), icloud.DriveEntryRequest{})
	case "delete_forever":
		value, err = runner.node.PermanentlyDelete(t.Context(), icloud.DriveEntryRequest{})
	default:
		t.Fatal("unknown node operation", call.Operation)
	}

	if err != nil {
		return nil, fmt.Errorf(replayNodeErrorFormat, err)
	}

	return portableDriveChanged(t, value), nil
}

func (runner *driveNodeRunner) upload(t *testing.T, call driveNodeCall) (any, error) {
	t.Helper()

	scenario := runner.scenario.driveUploadScenario
	scenario.Inputs = []json.RawMessage{json.RawMessage(`"unused-parent"`), call.Inputs[0]}

	scenario.Keywords = make(map[string]float64)

	for key, raw := range call.Keywords {
		var value float64

		err := json.Unmarshal(raw, &value)
		if err != nil {
			t.Fatal(err)
		}

		scenario.Keywords[key] = value
	}

	request, reader := sdkUploadRequest(t, scenario)
	value, err := runner.node.Upload(t.Context(), icloud.DriveUploadRequest{
		Content: reader, Filename: request.Filename, ModificationTime: request.ModificationTime,
		CreationTime: request.CreationTime})

	position, seekErr := reader.Seek(0, io.SeekCurrent)
	if seekErr != nil {
		t.Fatal(seekErr)
	}

	params := maps.Clone(scenario.Initial.Params)

	token := runner.session.Authentication().DriveToken
	if token != "" {
		params["token"] = token
	}

	state := map[string]any{"params": params, replayLiteralFilePosition: position}

	if err != nil {
		if len(scenario.ErrorState) != 0 {
			checkSDKValue(t, state, scenario.ErrorState)
		}

		return nil, fmt.Errorf(replayNodeErrorFormat, err)
	}

	start := runner.traffic.count - 3
	scenario.Exchanges = scenario.Exchanges[start:]
	checkSDKUploadResult(t, scenario, value)

	state["value"] = nil

	return state, nil
}
