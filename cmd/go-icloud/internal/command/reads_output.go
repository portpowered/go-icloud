package command

import "github.com/portpowered/go-icloud/pkg/icloud"

// Upload status is extensible on the wire. Only generated semantic fields reach
// ordinary output; the original map remains available in an explicit private file.
func typedReadProjection(result any) any {
	value, ok := result.(*icloud.GetPhotoUploadStatusResult)
	if !ok || value == nil {
		return result
	}
	copied := *value
	copied.Jobs = make(map[string]icloud.PhotoUploadStatus, len(value.Jobs))
	for id, status := range value.Jobs {
		status.AdditionalProperties = nil
		copied.Jobs[id] = status
	}
	return &copied
}
