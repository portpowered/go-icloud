package icloud

import "github.com/portpowered/go-icloud/pkg/dependencymodels/drive"

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
