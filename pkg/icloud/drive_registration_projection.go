package icloud

import "github.com/portpowered/go-icloud/pkg/dependencymodels/drive"

func projectDriveRegistrationStatus(status *drive.DriveRegistrationStatus) *DriveRegistrationStatus {
	if status == nil {
		return nil
	}

	return &DriveRegistrationStatus{StatusCode: status.StatusCode, ErrorMessage: status.ErrorMessage,
		AdditionalProperties: copyAccountMetadata(status.AdditionalProperties)}
}

func projectStructuredDriveRegistration(result *UploadDriveFileResult,
	registration *drive.DriveStructuredUpdatedDocuments,
) {
	result.StatusDetails = projectDriveRegistrationStatus(registration.Status)
	result.AdditionalRegistrationMetadata = copyAccountMetadata(registration.AdditionalProperties)

	if registration.Results == nil {
		return
	}

	results := make([]DriveRegistrationResult, 0, len(*registration.Results))
	documents := make([]RegisteredDriveDocument, 0, len(*registration.Results))

	for _, entry := range *registration.Results {
		projected := DriveRegistrationResult{StatusDetails: projectDriveRegistrationStatus(entry.Status), Document: nil,
			AdditionalProperties: copyAccountMetadata(entry.AdditionalProperties)}

		if entry.Document != nil {
			document := RegisteredDriveDocument{DocumentID: entry.Document.DocumentId, Status: nil,
				StatusDetails:        projectDriveRegistrationStatus(entry.Document.Status),
				AdditionalProperties: copyAccountMetadata(entry.Document.AdditionalProperties)}
			projected.Document = &document
			documents = append(documents, document)
		}

		results = append(results, projected)
	}

	result.Results = &results
	result.Documents = &documents
}
