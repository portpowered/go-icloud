package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

var errDevice = errors.New("provide --device with an identifier returned by findmy")

func controlFlags(flags *flag.FlagSet, config *options) {
	flags.StringVar(&config.deviceID, "device", "", "Discovered Find My device identifier")
	flags.StringVar(&config.phoneNumber, "phone", "", "Contact number for findmy-lost")
	flags.StringVar(&config.exportSession, "export-session", "", "Private destination for credentials-export")
	flags.Func("message", "Message for findmy-message or findmy-lost", func(value string) error {
		config.message = &value

		return nil
	})
	flags.Func("subject", "Alert subject for findmy-message or findmy-sound", func(value string) error {
		config.subject = &value

		return nil
	})
}

func controlFindMy(ctx context.Context, client icloud.Client,
	auth icloud.AuthContext, config options,
) (any, error) {
	if config.deviceID == "" {
		return nil, errDevice
	}

	session, err := client.OpenFindMySession(ctx, icloud.OpenFindMySessionRequest{Auth: auth,
		IncludeFamily: config.family}, icloud.WithFindMyMonitorInterval(0))
	if err != nil {
		return nil, fmt.Errorf("discover Find My devices: %w", err)
	}

	result, operationErr := findMyOperation(ctx, session, config)
	closeErr := session.Close()
	<-session.MonitorDone()

	err = errors.Join(operationErr, closeErr)
	if err != nil {
		return nil, fmt.Errorf("finish Find My operation: %w", err)
	}

	return result, nil
}

func findMyOperation(ctx context.Context, session *icloud.FindMySession, config options) (any, error) {
	switch config.operation {
	case "findmy-device":
		return wrap(session.DescribeDevice(icloud.FindMyDeviceDescriptionRequest{
			DeviceID: config.deviceID, AdditionalStatus: nil}))
	case "findmy-sound":
		return wrap(session.PlaySound(ctx, icloud.FindMySoundRequest{DeviceID: config.deviceID, Subject: config.subject}))
	case "findmy-message":
		return wrap(session.SendMessage(ctx, icloud.FindMyMessageRequest{DeviceID: config.deviceID,
			Subject: config.subject, Text: config.message, Sound: false, Strobe: false, Vibrate: false}))
	case "findmy-lost":
		return wrap(session.MarkLost(ctx, icloud.FindMyLostRequest{DeviceID: config.deviceID,
			PhoneNumber: config.phoneNumber, Text: config.message, Passcode: os.Getenv("GO_ICLOUD_DEVICE_PASSCODE")}))
	case "findmy-erase":
		return wrap(session.Erase(ctx, icloud.FindMyEraseRequest{DeviceID: config.deviceID,
			Text: config.message, Passcode: os.Getenv("GO_ICLOUD_DEVICE_PASSCODE")}))
	default:
		return nil, errCommand
	}
}
