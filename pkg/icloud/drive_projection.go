package icloud

import (
	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/drive"
)

func projectDriveNode(node drive.DriveNode) DriveNode {
	result := DriveNode{
		DateChanged: node.DateChanged, DateModified: node.DateModified, Docwsid: node.Docwsid,
		Drivewsid: node.Drivewsid, Etag: node.Etag, Extension: node.Extension,
		Items: nil, LastOpenTime: node.LastOpenTime, Name: node.Name, RestorePath: node.RestorePath,
		ShareID: nil, Size: node.Size, Status: node.Status, Type: node.Type, Zone: node.Zone,
		AdditionalProperties: copyAccountMetadata(node.AdditionalProperties),
	}

	if node.Items != nil {
		items := projectDriveNodes(*node.Items)
		result.Items = &items
	}

	if node.ShareID != nil {
		share := DriveShareID(*node.ShareID)
		share.AdditionalProperties = copyAccountMetadata(node.ShareID.AdditionalProperties)
		result.ShareID = &share
	}

	return result
}

func projectDriveNodes(nodes []drive.DriveNode) []DriveNode {
	result := make([]DriveNode, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, projectDriveNode(node))
	}

	return result
}

func projectDriveOptionalNodes(nodes *[]drive.DriveNode) *[]DriveNode {
	if nodes == nil {
		return nil
	}

	result := projectDriveNodes(*nodes)

	return &result
}

func projectDriveChanged(response *webtransport.DriveChangedResponse) *DriveItemChangeResult {
	return &DriveItemChangeResult{Items: projectDriveOptionalNodes(response.Data.Items),
		AdditionalMetadata: copyAccountMetadata(response.Data.AdditionalProperties),
		Metadata:           publicMetadata(response.Response)}
}

func projectDriveCreated(response *webtransport.DriveCreatedResponse) *CreateDriveFolderResult {
	return &CreateDriveFolderResult{Folders: projectDriveOptionalNodes(response.Data.Folders),
		AdditionalMetadata: copyAccountMetadata(response.Data.AdditionalProperties),
		Metadata:           publicMetadata(response.Response)}
}
