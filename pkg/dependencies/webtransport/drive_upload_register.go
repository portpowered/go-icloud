package webtransport

import (
	"bytes"
	"context"
	"path/filepath"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/driveapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/drive"
)

func uploadTimestamp(value *time.Time, clock func() time.Time) int64 {
	if value != nil {
		return value.UnixMilli()
	}

	return clock().UnixMilli()
}

func uploadDocument(input DriveUploadInput, documentID string, file drive.DriveUploadedFile) drive.DriveUpdateDocument {
	data := drive.DriveDocumentData{Signature: file.FileChecksum, WrappingKey: file.WrappingKey,
		ReferenceSignature: file.ReferenceChecksum, Size: file.Size, Receipt: nil}
	if file.Receipt != nil && *file.Receipt != "" {
		data.Receipt = file.Receipt
	}

	return drive.DriveUpdateDocument{Data: data, Command: drive.AddFile,
		CreateShortGuid: drive.DriveUpdateDocumentCreateShortGuidTrue, DocumentId: documentID,
		Path:          drive.DriveDocumentPath{StartingDocumentId: input.ParentID, Path: filepath.Base(input.Filename)},
		AllowConflict: drive.DriveUpdateDocumentAllowConflictTrue,
		FileFlags: drive.DriveFileFlags{IsWritable: drive.DriveFileFlagsIsWritableTrue,
			IsExecutable: drive.DriveFileFlagsIsExecutableFalse, IsHidden: drive.DriveFileFlagsIsHiddenFalse},
		Mtime: uploadTimestamp(input.ModificationTime, input.Clock), Btime: uploadTimestamp(input.CreationTime, input.Clock)}
}

func (client *Client) registerDriveUpload(ctx context.Context, auth RequestContext, input DriveUploadInput,
	documentID, token string, file drive.DriveUploadedFile,
) (*BytesResponse, drive.DriveUpdatedDocuments, error) {
	var data drive.DriveUpdatedDocuments

	body, err := referenceJSON(uploadDocument(input, documentID, file))
	if err != nil {
		return nil, data, failure(Configuration, err, nil, nil)
	}

	params := driveapi.DriveRegisterDocumentParams{ClientId: auth.Params.ClientId, Dsid: auth.Params.Dsid,
		ClientBuildNumber: auth.Params.ClientBuildNumber, ClientMasteringNumber: auth.Params.ClientMasteringNumber,
		Token: &token, Accept: nil, Cookie: nil, Origin: nil, Referer: nil, UserAgent: nil,
		AcceptEncoding: nil, Connection: nil}

	request, err := driveapi.NewDriveRegisterDocumentRequestWithBody(auth.Origin, input.Zone, &params,
		plainTextMedia(), bytes.NewReader(body))
	if err != nil {
		return nil, data, failure(Configuration, err, nil, nil)
	}

	response, err := client.readWithPolicy(ctx, plainTextUploadAuth(auth), request,
		"&"+queryPart(protocol.DriveRegisterDocumentTokenName, token), successfulContent, true)
	if err != nil {
		return nil, data, err
	}

	err = decodeDriveObject(response.Body, &data)
	if err != nil {
		return nil, data, responseFailure(Decode, err, response)
	}

	return response, data, nil
}
