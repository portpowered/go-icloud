package contracts_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const consumerSource = `package consumer

import (
 "context"
 "errors"
 "io"
 "github.com/portpowered/go-icloud/pkg/icloud"
)

var _ icloud.Client = (*icloud.SDK)(nil)

func Devices(ctx context.Context, auth icloud.AuthContext) (*icloud.GetAccountDevicesResult, error) {
 client, err := icloud.New()
 if err != nil { return nil, err }
 result, err := client.GetAccountDevices(ctx, icloud.GetAccountDevicesRequest{Auth: auth})
 var failure *icloud.ClientError
 if errors.As(err, &failure) {
  _ = failure.Kind()
  _ = failure.StatusCode()
  _ = failure.ResponseHeaders()
  _ = failure.ResponseBody()
  _ = failure.PriorResponses()
  _ = failure.CookieScopeURL()
  _ = failure.UploadToken()
 }
 return result, err
}
func Account(ctx context.Context, auth icloud.AuthContext, memberID string) error {
 client, err := icloud.New()
 if err != nil { return err }
 _, err = client.GetAccountFamily(ctx, icloud.GetAccountFamilyRequest{Auth: auth})
 if err != nil { return err }
 _, err = client.GetAccountMemberPhoto(ctx, icloud.GetAccountMemberPhotoRequest{Auth: auth, MemberID: memberID})
 if err != nil { return err }
 _, err = client.GetAccountStorage(ctx, icloud.GetAccountStorageRequest{Auth: auth})
 if err != nil { return err }
 _, err = client.GetAccountPlanSummary(ctx, icloud.GetAccountPlanSummaryRequest{Auth: auth})
 return err
}

func Drive(ctx context.Context, auth icloud.AuthContext, nodeID string) error {
 client, err := icloud.New()
 if err != nil { return err }
 _, err = client.GetDriveNode(ctx, icloud.GetDriveNodeRequest{Auth: auth, NodeID: nodeID})
 if err != nil { return err }
 _, err = client.ListDriveLibraries(ctx, icloud.ListDriveLibrariesRequest{Auth: auth})
 if err != nil { return err }
 _, err = client.DownloadDriveFile(ctx, icloud.DownloadDriveFileRequest{Auth: auth, DocumentID: nodeID})
 return err
}


func Upload(ctx context.Context, auth icloud.AuthContext, parentID, filename string, content io.ReadSeeker) error {
 client, err := icloud.New()
 if err != nil { return err }
 result, err := client.UploadDriveFile(ctx, icloud.UploadDriveFileRequest{
  Auth: auth, ParentID: parentID, Filename: filename, Content: content,
 })
 if result != nil {
  _ = result.UploadToken
  _ = result.PreparationMetadata.CookieScopeURL
  _ = result.TransferMetadata
  _ = result.RegistrationMetadata
 }
 return err
}

func DriveSessionFlows(ctx context.Context, auth icloud.AuthContext, content io.ReadSeeker) error {
 client, err := icloud.New()
 if err != nil { return err }
 session, err := client.OpenDriveSession(ctx, icloud.OpenDriveSessionRequest{Auth: auth})
 if err != nil { return err }
 defer session.Close()
 _ = session.Authentication().DriveToken
 _ = session.LastResponses()
 root, err := session.Root(ctx, icloud.DriveLocationRequest{})
 if err != nil { return err }
 _, err = session.Trash(ctx, icloud.DriveLocationRequest{Refresh: true})
 if err != nil { return err }
 _, err = session.Directory(ctx, icloud.DriveEntryRequest{})
 if err != nil { return err }
 _, err = session.Lookup(ctx, icloud.DriveLookupRequest{Name: "Synthetic.txt"})
 if err != nil { return err }
 _, err = root.Snapshot()
 if err != nil { return err }
 _, err = root.Children(ctx, icloud.DriveChildrenRequest{Force: true})
 if err != nil { return err }
 _, err = root.Directory(ctx, icloud.DriveEntryRequest{})
 if err != nil { return err }
 _, err = root.Lookup(ctx, icloud.DriveLookupRequest{Name: "Synthetic.txt"})
 if err != nil { return err }
 _, err = root.CreateFolder(ctx, icloud.DriveFolderRequest{Name: "Synthetic"})
 if err != nil { return err }
 _, err = root.Rename(ctx, icloud.DriveRenameRequest{Name: "Synthetic"})
 if err != nil { return err }
 _, err = root.Trash(ctx, icloud.DriveEntryRequest{})
 if err != nil { return err }
 _, err = root.Delete(ctx, icloud.DriveEntryRequest{})
 if err != nil { return err }
 _, err = root.Restore(ctx, icloud.DriveEntryRequest{})
 if err != nil { return err }
 _, err = root.PermanentlyDelete(ctx, icloud.DriveEntryRequest{})
 if err != nil { return err }
 _, err = root.Download(ctx, icloud.DriveEntryRequest{})
 if err != nil { return err }
 _, err = root.Upload(ctx, icloud.DriveUploadRequest{Filename: "Synthetic.txt", Content: content})
 return err
}

func DriveMutations(ctx context.Context, auth icloud.AuthContext, node icloud.DriveNodeSelector) error {
 client, err := icloud.New()
 if err != nil { return err }
 _, err = client.CreateDriveFolder(ctx, icloud.CreateDriveFolderRequest{
 Auth: auth, ParentID: node.NodeID, Name: "Synthetic",
 })
 if err != nil { return err }
 _, err = client.RenameDriveNode(ctx, icloud.RenameDriveNodeRequest{Auth: auth, Node: node, Name: "Synthetic"})
 if err != nil { return err }
 _, err = client.MoveDriveNodes(ctx, icloud.MoveDriveNodesRequest{Auth: auth, DestinationID: node.NodeID, Nodes: nil})
 if err != nil { return err }
 _, err = client.TrashDriveNode(ctx, icloud.TrashDriveNodeRequest{Auth: auth, Node: node})
 if err != nil { return err }
 _, err = client.RestoreDriveNode(ctx, icloud.RestoreDriveNodeRequest{Auth: auth, Node: node})
 if err != nil { return err }
 _, err = client.DeleteDriveNode(ctx, icloud.DeleteDriveNodeRequest{Auth: auth, Node: node})
 if err != nil { return err }
 _, err = client.PermanentlyDeleteDriveNode(ctx, icloud.PermanentlyDeleteDriveNodeRequest{Auth: auth, Node: node})
 return err
}


func FindMy(ctx context.Context, auth icloud.AuthContext, deviceID string) error {
 client, err := icloud.New()
 if err != nil { return err }
 session, err := client.OpenFindMySession(ctx, icloud.OpenFindMySessionRequest{Auth: auth},
  icloud.WithFindMyMonitorInterval(0))
 if err != nil { return err }
 defer session.Close()
 _ = session.Authentication().SessionToken
 _ = session.LastResponses()
 _ = session.LastError()
 _ = session.MonitorDone()
 _, err = session.Snapshot()
 if err != nil { return err }
 description, err := session.DescribeDevice(icloud.FindMyDeviceDescriptionRequest{
  DeviceID: deviceID, AdditionalStatus: []string{"future"},
 })
 if err != nil { return err }
 _ = description.Capabilities.Location
 _ = description.Status.BatteryLevel
 _, err = session.Refresh(ctx, icloud.RefreshFindMyRequest{Locate: true})
 if err != nil { return err }
 _, err = session.PlaySound(ctx, icloud.FindMySoundRequest{DeviceID: deviceID})
 if err != nil { return err }
 _, err = session.SendMessage(ctx, icloud.FindMyMessageRequest{DeviceID: deviceID})
 if err != nil { return err }
 _, err = session.MarkLost(ctx, icloud.FindMyLostRequest{DeviceID: deviceID, PhoneNumber: "synthetic"})
 if err != nil { return err }
 result, err := session.Erase(ctx, icloud.FindMyEraseRequest{DeviceID: deviceID})
 if result != nil { _ = result.Responses; _ = result.Acknowledgement }
 return err
}

`

func TestPublicSDKCompilesInIndependentConsumer(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	directory := t.TempDir()
	module := fmt.Sprintf("module example.invalid/consumer\n\ngo 1.24.0\n\n"+
		"require github.com/portpowered/go-icloud v0.0.0\n"+
		"replace github.com/portpowered/go-icloud => %q\n", filepath.ToSlash(root))

	for name, source := range map[string]string{"go.mod": module, "consumer.go": consumerSource} {
		err = os.WriteFile(filepath.Join(directory, name), []byte(source), contractFileMode)
		if err != nil {
			t.Fatal(err)
		}
	}

	command := exec.CommandContext(t.Context(), "go", "build", "-mod=mod", ".")
	command.Dir = directory

	// API-02: build only; normal module resolution is allowed on a clean consumer cache.
	command.Env = append(os.Environ(), "GOWORK=off")

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("independent SDK consumer: %v: %s", err, output)
	}
}
