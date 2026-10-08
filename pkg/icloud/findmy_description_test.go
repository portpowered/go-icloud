package icloud_test

import (
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestFindMyDescriptionRetainsCopiedCacheAfterClose(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	session := openFindMy(t, func(_ *http.Request) (*http.Response, error) {
		calls.Add(1)

		return findMyReply(http.StatusOK, `{"content":[{"id":"device","future":[null,true]}]}`), nil
	}, icloud.WithFindMyMonitorInterval(0))

	err := session.Close()
	if err != nil {
		t.Fatal(err)
	}

	request := icloud.FindMyDeviceDescriptionRequest{DeviceID: "device", AdditionalStatus: []string{"future"}}

	description, err := session.DescribeDevice(request)
	if err != nil {
		t.Fatal(err)
	}

	description.Device.AdditionalProperties["future"][0] = '!'
	description.Status.AdditionalProperties["future"][0] = '!'

	again, err := session.DescribeDevice(request)
	if err != nil || again.Device.AdditionalProperties["future"][0] != '[' ||
		again.Status.AdditionalProperties["future"][0] != '[' || calls.Load() != 1 {
		t.Fatal("local device description exposed state or performed a request after close")
	}

	_, err = session.DescribeDevice(icloud.FindMyDeviceDescriptionRequest{
		DeviceID: "not-discovered", AdditionalStatus: nil})
	assertSDKKind(t, err, icloud.NotFound)
}
