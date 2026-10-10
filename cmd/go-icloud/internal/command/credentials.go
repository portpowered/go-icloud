package command

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/commandmodels"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

var errExportDestination = errors.New("provide --export-session with a private destination")

type exportSummary = commandmodels.ExportSummary

func exportCredentials(ctx context.Context, config options, output io.Writer) error {
	if config.exportSession == "" {
		return errExportDestination
	}

	data, err := os.ReadFile(filepath.Clean(config.session))
	if err != nil {
		return &SessionError{Cause: err}
	}

	var saved icloud.ResumeSessionResult

	err = json.Unmarshal(data, &saved)
	if err != nil {
		return &SessionError{Cause: err}
	}

	_, err = loadSession(config.session)
	if err != nil {
		return err
	}

	err = savePrivateData(ctx, config.exportSession, data)
	if err != nil {
		return &SessionError{Cause: err}
	}

	return writeResult(output, exportSummary{SessionFile: config.exportSession})
}
