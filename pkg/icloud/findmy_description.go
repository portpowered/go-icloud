package icloud

import (
	"encoding/json"
	"fmt"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/internal/protocol"
)

// DescribeDevice returns copied cached metadata, capabilities, location and selected status fields.
// It performs no network request and remains available after Close; call Refresh for fresh data.
func (session *FindMySession) DescribeDevice(request FindMyDeviceDescriptionRequest) (*FindMyDeviceDescription, error) {
	const operation = "FindMyDescribeDevice"

	snapshot, err := session.Snapshot()
	if err != nil {
		return nil, err
	}

	for _, device := range snapshot.Devices {
		if device.Id != request.DeviceID {
			continue
		}

		result, err := describeFindMyDevice(device, request.AdditionalStatus)
		if err != nil {
			return nil, newClientError(operation, InvalidResponse, 0, nil, nil, err)
		}

		return result, nil
	}

	return nil, newClientError(operation, NotFound, 0, nil, nil, errFindMyDevice)
}

func describeFindMyDevice(device FindMyDevice, additional []string) (*FindMyDeviceDescription, error) {
	capabilities := FindMyCapabilities{Sound: false, Messaging: false, Erase: false,
		LostMode: findMyFlag(device.LostModeCapable), Location: false}
	if device.Features != nil {
		capabilities.Sound = findMyFlag(device.Features.SND)
		capabilities.Messaging = findMyFlag(device.Features.MSG)
		capabilities.Erase = findMyFlag(device.Features.WIP)
		capabilities.Location = findMyFlag(device.Features.LOC) && device.Location.IsSpecified() && !device.Location.IsNull()
	}

	location := nullable.NewNullNullable[FindMyLocation]()
	if capabilities.Location {
		location = device.Location
	}

	status, err := findMyDeviceStatus(device, additional)
	if err != nil {
		return nil, err
	}

	return &FindMyDeviceDescription{Device: device, Name: findMyText(device.Name, ""),
		Model: findMyText(device.DeviceModel, ""), ModelName: findMyText(device.DeviceDisplayName, ""),
		DeviceType: findMyText(device.DeviceClass, ""), Location: location, Capabilities: capabilities, Status: *status}, nil
}

func findMyDeviceStatus(device FindMyDevice, additional []string) (*FindMyDeviceStatus, error) {
	body, err := json.Marshal(device)
	if err != nil {
		return nil, fmt.Errorf("encode cached Find My device: %w", err)
	}

	fields := make(map[string]json.RawMessage)

	err = json.Unmarshal(body, &fields)
	if err != nil {
		return nil, fmt.Errorf("decode cached Find My fields: %w", err)
	}

	defaults := []string{protocol.FindMyDeviceBatteryLevel, protocol.FindMyDeviceDeviceDisplayName,
		protocol.FindMyDeviceDeviceStatus, protocol.FindMyDeviceName}
	keys := make([]string, 0, len(defaults)+len(additional))
	keys = append(keys, defaults...)
	keys = append(keys, additional...)

	values := make(map[string]json.RawMessage, len(keys))

	for _, key := range keys {
		value, exists := fields[key]
		if !exists {
			value = json.RawMessage("null")
		}

		values[key] = value
	}

	result := new(FindMyDeviceStatus)

	err = projectFindMyValue(values, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
