package replay_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func driveMutationOperations() map[string]bool {
	return map[string]bool{"create_folders": true, "rename_items": true, "move_nodes_to_node": true,
		"delete_items": true, "delete_forever_from_trash": true,
		"move_items_to_trash": true, "recover_items_from_trash": true}
}

func TestDriveSDKPortableMutations(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/drive-*.json")
	if err != nil {
		t.Fatal(err)
	}

	selected := 0
	operations := driveMutationOperations()

	for _, path := range paths {
		data, readErr := os.ReadFile(filepath.Clean(path))
		if readErr != nil {
			t.Fatal(readErr)
		}

		var scenario driveReadScenario

		decodeErr := json.Unmarshal(data, &scenario)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}

		if !operations[scenario.Operation] {
			continue
		}

		selected++

		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runDriveSDKMutation(t, scenario)
		})
	}

	if selected != 16 {
		t.Fatal("Drive mutation scenario inventory changed")
	}
}

func runDriveSDKMutation(t *testing.T, scenario driveReadScenario) {
	t.Helper()

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	result, metadata, err := callDriveSDKMutation(t, client, scenario)
	if len(scenario.Error) != 0 {
		assertDriveSDKFailure(t, scenario, err)
	} else {
		assertDriveSDKSuccess(t, scenario, result, metadata, err)
	}

	consumeErr := transport.AssertConsumed()
	if consumeErr != nil {
		t.Fatal(consumeErr)
	}
}

func callDriveSDKMutation(t *testing.T, client icloud.Client,
	scenario driveReadScenario,
) (any, icloud.ResponseMetadata, error) {
	t.Helper()

	auth := sdkAccountAuth(scenario.Initial)
	auth.DriveServiceURL = scenario.Initial.Origin
	auth.AccountServiceURL = ""

	if scenario.Operation == "create_folders" {
		result, err := client.CreateDriveFolder(t.Context(), icloud.CreateDriveFolderRequest{
			Auth: auth, ParentID: driveStringInput(t, scenario.Inputs, 0), Name: driveStringInput(t, scenario.Inputs, 1),
		})
		if err != nil {
			return nil, icloud.ResponseMetadata{}, fmt.Errorf("Drive folder creation: %w", err)
		}

		return driveMutationEnvelope(t, result.AdditionalMetadata, protocol.DriveCreatedFoldersFolders, result.Folders),
			result.Metadata, nil
	}

	result, err := callDriveItemMutation(t, client, auth, scenario)
	if err != nil {
		return nil, icloud.ResponseMetadata{}, fmt.Errorf("Drive item mutation: %w", err)
	}

	return driveMutationEnvelope(t, result.AdditionalMetadata, protocol.DriveChangedItemsItems, result.Items),
		result.Metadata, nil
}

func callDriveItemMutation(t *testing.T, client icloud.Client,
	auth icloud.AuthContext, scenario driveReadScenario,
) (*icloud.DriveItemChangeResult, error) {
	t.Helper()

	if scenario.Operation == "move_nodes_to_node" {
		return callDriveMove(t, client, auth, scenario)
	}

	node := icloud.DriveNodeSelector{NodeID: driveStringInput(t, scenario.Inputs, 0),
		ETag: driveStringInput(t, scenario.Inputs, 1)}

	var result *icloud.DriveItemChangeResult

	var err error

	switch scenario.Operation {
	case "rename_items":
		result, err = client.RenameDriveNode(t.Context(), icloud.RenameDriveNodeRequest{
			Auth: auth, Node: node, Name: driveStringInput(t, scenario.Inputs, 2),
		})
	case "move_items_to_trash":
		result, err = client.TrashDriveNode(t.Context(), icloud.TrashDriveNodeRequest{Auth: auth, Node: node})
	case "recover_items_from_trash":
		result, err = client.RestoreDriveNode(t.Context(), icloud.RestoreDriveNodeRequest{Auth: auth, Node: node})
	case "delete_items":
		result, err = client.DeleteDriveNode(t.Context(), icloud.DeleteDriveNodeRequest{Auth: auth, Node: node})
	case "delete_forever_from_trash":
		result, err = client.PermanentlyDeleteDriveNode(t.Context(), icloud.PermanentlyDeleteDriveNodeRequest{
			Auth: auth, Node: node,
		})
	default:
		t.Fatal("undeclared Drive mutation operation")
	}

	if err != nil {
		return nil, fmt.Errorf("Drive selected item: %w", err)
	}

	return result, nil
}

func callDriveMove(t *testing.T, client icloud.Client,
	auth icloud.AuthContext, scenario driveReadScenario,
) (*icloud.DriveItemChangeResult, error) {
	t.Helper()

	var nodes []map[string]string

	err := json.Unmarshal(scenario.Inputs[0], &nodes)
	if err != nil {
		t.Fatal(err)
	}

	var destination map[string]string

	err = json.Unmarshal(scenario.Inputs[1], &destination)
	if err != nil {
		t.Fatal(err)
	}

	request := icloud.MoveDriveNodesRequest{Auth: auth,
		DestinationID: destination[protocol.DriveNodeDrivewsid], Nodes: make([]icloud.DriveNodeSelector, 0, len(nodes))}
	for _, node := range nodes {
		request.Nodes = append(request.Nodes, icloud.DriveNodeSelector{
			NodeID: node[protocol.DriveNodeDrivewsid], ETag: node[protocol.DriveNodeEtag],
		})
	}

	result, err := client.MoveDriveNodes(t.Context(), request)
	if err != nil {
		return nil, fmt.Errorf("Drive move selection: %w", err)
	}

	return result, nil
}

func driveStringInput(t *testing.T, inputs []json.RawMessage, index int) string {
	t.Helper()

	var value string

	err := json.Unmarshal(inputs[index], &value)
	if err != nil {
		t.Fatal(err)
	}

	return value
}

func driveMutationEnvelope(t *testing.T, additional map[string]json.RawMessage,
	name string, nodes *[]icloud.DriveNode,
) map[string]json.RawMessage {
	t.Helper()

	result := make(map[string]json.RawMessage)
	maps.Copy(result, additional)

	if nodes != nil {
		encoded, err := json.Marshal(*nodes)
		if err != nil {
			t.Fatal(err)
		}

		result[name] = encoded
	}

	return result
}
