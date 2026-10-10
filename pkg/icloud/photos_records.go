package icloud

import (
	"fmt"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func photoNormalRecords(data cloudkit.CKQueryResponse) ([]cloudkit.CKRecord, error) {
	records := []cloudkit.CKRecord{}
	if data.Records == nil {
		return records, nil
	}

	for _, item := range *data.Records {
		record, err := webtransport.DecodeReminderQueryRecord(item)
		if err != nil {
			return nil, fmt.Errorf("decode photo record: %w", err)
		}

		if record.Record != nil {
			if record.Record.ExpirationTime.IsSpecified() && !record.Record.ExpirationTime.IsNull() {
				_, err := photoExpirationValue(record.Record.ExpirationTime.GetOrEmpty())
				if err != nil {
					return nil, err
				}
			}

			records = append(records, *record.Record)
		}
	}

	return records, nil
}
