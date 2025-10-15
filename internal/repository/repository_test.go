package repository

import (
	"path/filepath"
	"testing"

	compare "github.com/kilianpaquier/compare/pkg"
	"github.com/stretchr/testify/require"
)

func TestLayoutFromTUFOnCI(t *testing.T) {
	testCases := []struct {
		name        string
		expectedErr string
	}{{
		name: "simple",
	}, {
		name: "delegated",
	}, {
		name: "no-files",
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tufOnCIRoot := filepath.Join("testdata", tc.name, "tuf-on-ci")

			outputRoot := t.TempDir()

			err := LayoutFromTUFOnCI(tufOnCIRoot, outputRoot)

			if tc.expectedErr == "" {
				require.NoError(t, err)

				expectedOutputDir := filepath.Join("testdata", tc.name, "output")

				require.NoError(t, compare.Dirs(expectedOutputDir, outputRoot))
			} else {
				require.ErrorContains(t, err, tc.expectedErr)
			}
		})
	}
}
