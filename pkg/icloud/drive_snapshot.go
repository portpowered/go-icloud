package icloud

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
)

func copyDriveData(input DriveNode) (DriveNode, error) {
	var output DriveNode

	data, err := json.Marshal(input)
	if err != nil {
		return output, fmt.Errorf("copy Drive value: %w", err)
	}

	err = json.Unmarshal(data, &output)
	if err != nil {
		return output, fmt.Errorf("decode copied Drive data: %w", err)
	}

	return output, nil
}

func cloneDriveAuth(auth AuthContext) AuthContext {
	auth.Headers = append([]Header(nil), auth.Headers...)
	auth.SessionToken = copyString(auth.SessionToken)
	auth.ClientBuildNumber = copyString(auth.ClientBuildNumber)
	auth.ClientMasteringNumber = copyString(auth.ClientMasteringNumber)
	auth.ChinaMainland = cloneDrivePointer(auth.ChinaMainland)

	auth.Cookies = append([]AuthCookie(nil), auth.Cookies...)

	for index := range auth.Cookies {
		auth.Cookies[index].Expires = cloneDrivePointer(auth.Cookies[index].Expires)
		auth.Cookies[index].SameSite = cloneDrivePointer(auth.Cookies[index].SameSite)
	}

	return auth
}

func cloneDrivePointer[Value any](value *Value) *Value {
	if value == nil {
		return nil
	}

	copyValue := *value

	return &copyValue
}

func driveSnapshot(data DriveNode) (*DriveEntrySnapshot, error) {
	size, err := driveSize(data.Size)
	if err != nil {
		return nil, newClientError("DriveSnapshot", InvalidResponse, 0, nil, nil, err)
	}

	return &DriveEntrySnapshot{Data: data, Name: driveName(data), Type: driveType(data), Size: size,
		DateChanged: driveUTC(data.DateChanged), DateModified: driveUTC(data.DateModified),
		DateLastOpen: driveUTC(data.LastOpenTime)}, nil
}

func driveName(data DriveNode) string {
	name := driveString(data.Name)
	if name == "" {
		name = driveString(data.Drivewsid)
		if name == protocol.DriveRootIdentifierValue {
			name = "root"
		}

		if name == "" {
			name = "<UNKNOWN>"
		}
	}

	if data.Extension != nil {
		name += "." + *data.Extension
	}

	return name
}

func driveType(data DriveNode) string {
	value := driveString(data.Type)
	if value == "" && driveString(data.Drivewsid) == protocol.DriveTrashNodeIdentifierValue {
		value = "trash"
	}

	if value == "" {
		value = "unknown"
	}

	return strings.ToLower(value)
}

func driveString(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func driveSize(value *json.RawMessage) (*int64, error) {
	if value == nil || string(*value) == "0" || string(*value) == `""` {
		//nolint:nilnil // SCHEMA-04: absence is a valid optional size property.
		return nil, nil
	}

	text := string(*value)
	if strings.HasPrefix(text, `"`) {
		err := json.Unmarshal(*value, &text)
		if err != nil {
			return nil, fmt.Errorf("decode Drive size: %w", err)
		}
	}

	size, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("decode Drive size: %w", err)
	}

	return &size, nil
}

func driveUTC(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	// Pinned Source subtracts signed hours and unsigned minutes separately.
	// Preserve that observable arithmetic even for a negative fractional-hour offset.
	_, offset := value.Zone()

	const secondsPerHour = 3600

	hours, remainder := offset/secondsPerHour, offset%secondsPerHour
	if remainder < 0 {
		remainder = -remainder
	}

	base := time.Date(value.Year(), value.Month(), value.Day(), value.Hour(), value.Minute(), value.Second(),
		value.Nanosecond(), time.UTC)
	utc := base.Add(-time.Duration(hours*secondsPerHour+remainder) * time.Second)

	return &utc
}
