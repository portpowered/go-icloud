package webtransport

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/drive"
)

func requireUploadFields(body []byte, names ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage

	err := json.Unmarshal(body, &fields)
	if err != nil {
		return nil, fmt.Errorf("decode upload object: %w", err)
	}

	for _, name := range names {
		value := bytes.TrimSpace(fields[name])
		if len(value) == 0 || bytes.Equal(value, []byte("null")) {
			return nil, errDriveShape
		}
	}

	return fields, nil
}

func decodeUploadDestination(body []byte) (drive.DriveUploadDestination, error) {
	var result drive.DriveUploadDestination

	var entries []json.RawMessage

	err := json.Unmarshal(body, &entries)
	if err != nil || len(entries) == 0 {
		return result, errDriveShape
	}

	_, err = requireUploadFields(entries[0], protocol.DriveUploadDestinationDocumentId,
		protocol.DriveUploadDestinationUrl)
	if err != nil {
		return result, err
	}

	err = json.Unmarshal(entries[0], &result)
	if err != nil {
		return result, fmt.Errorf("decode upload destination: %w", err)
	}

	return result, nil
}

func decodeUploadReceipt(body []byte) (drive.DriveUploadReceipt, error) {
	var result drive.DriveUploadReceipt

	fields, err := requireUploadFields(body, protocol.DriveUploadReceiptSingleFile)
	if err != nil {
		return result, err
	}

	_, err = requireUploadFields(fields[protocol.DriveUploadReceiptSingleFile],
		protocol.DriveUploadedFileFileChecksum, protocol.DriveUploadedFileWrappingKey,
		protocol.DriveUploadedFileReferenceChecksum, protocol.DriveUploadedFileSize)
	if err != nil {
		return result, err
	}

	err = decodeDriveObject(body, &result)
	if err != nil {
		return result, err
	}

	return result, nil
}
