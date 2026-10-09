package command

import (
	"flag"
	"fmt"
	"strconv"
)

func reminderQueryFlags(flags *flag.FlagSet, config *options) {
	flags.StringVar(&config.listID, "list", "", "Literal list identifier; optional for reminder-snapshot")
	flags.BoolVar(&config.includeCompleted, "include-completed", false, "Include completed reminders")
	flags.Func("page-size", "Optional provider page size for reminders; all pages are consumed", func(value string) error {
		limit, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("parse reminder page size: %w", err)
		}

		config.resultsLimit = &limit

		return nil
	})
}
