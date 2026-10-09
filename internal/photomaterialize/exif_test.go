package photomaterialize_test

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/internal/photomaterialize"
)

func TestSourceTIFFVariantOracle(t *testing.T) {
	t.Parallel()

	oracle := readOracle(t)
	if oracle.ExifCases == nil || len(*oracle.ExifCases) == 0 {
		t.Fatal("Source TIFF oracle cases are absent")
	}

	for _, variant := range *oracle.ExifCases {
		t.Run(variant.Name, func(t *testing.T) {
			t.Parallel()

			input, err := hex.DecodeString(variant.InputHex)
			if err != nil {
				t.Fatal(err)
			}

			stamp := time.Date(2024, 1, 2, 4, 4, 5, 0, time.UTC)

			actual := photomaterialize.UpdateEXIF(input, stamp)
			if hex.EncodeToString(actual) != variant.OutputHex {
				t.Fatalf("Source TIFF variant differs: %x", actual)
			}
		})
	}
}
