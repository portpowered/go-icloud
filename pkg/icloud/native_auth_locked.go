package icloud

import (
	"encoding/json"
	"strconv"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

func nativeLockedBody(body []byte) bool {
	var data auth.AuthFailure
	if json.Unmarshal(body, &data) != nil || data.ServiceErrors == nil {
		return false
	}
	for _, item := range *data.ServiceErrors {
		if item.Code == nil {
			continue
		}
		text, err := item.Code.AsAuthServiceErrorCode0()
		if err == nil && text == strconv.Itoa(int(auth.AccountLockedCode)) {
			return true
		}
		number, err := item.Code.AsAuthServiceErrorCode1()
		if err == nil && number == int(auth.AccountLockedCode) {
			return true
		}
	}
	return false
}
