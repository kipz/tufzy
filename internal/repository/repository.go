package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// LayoutFromTUFOnCI takes the path to a tuf-on-ci generated layout of metadata and targets, and copies the appropriate
// metadata and target files from there into a standard TUF root layout.
func LayoutFromTUFOnCI(tufOnCIPath string, outputDir string) error {
	// Make sure the output directory and its metadata subdirectory exist.
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("couldn't create root output directory %s: %w", outputDir, err)
	}

	outputMetadataDir := filepath.Join(outputDir, "metadata")
	if err := os.MkdirAll(outputMetadataDir, 0755); err != nil {
		return fmt.Errorf("couldn't create metadata output directory %s: %w", outputMetadataDir, err)
	}

	metadataDir := filepath.Join(tufOnCIPath, "metadata")
	targetsDir := filepath.Join(tufOnCIPath, "targets")

	// Copy the root_history/*.root.json files.
	rootHistoryPath := filepath.Join(metadataDir, "root_history")

	historyFiles, err := os.ReadDir(rootHistoryPath)
	if err != nil {
		return fmt.Errorf("reading root history from %s: %w", rootHistoryPath, err)
	}

	for _, rh := range historyFiles {
		if strings.HasSuffix(rh.Name(), ".root.json") {
			if err := copyFile(filepath.Join(rootHistoryPath, rh.Name()), filepath.Join(outputMetadataDir, rh.Name())); err != nil {
				return err
			}
		}
	}

	// Copy the timestamp.json
	if err := copyFile(filepath.Join(metadataDir, "timestamp.json"), filepath.Join(outputMetadataDir, "timestamp.json")); err != nil {
		return err
	}

	// Read the snapshot.json and copy it.
	snapshotFile := filepath.Join(metadataDir, "snapshot.json")
	snapshot, err := metadata.Snapshot().FromFile(snapshotFile)
	if err != nil {
		return fmt.Errorf("loading snapshot from %s: %w", snapshotFile, err)
	}

	outputSnapshotFile := filepath.Join(outputMetadataDir, fmt.Sprintf("%d.snapshot.json", snapshot.Signed.Version))
	if err := copyFile(snapshotFile, outputSnapshotFile); err != nil {
		return err
	}

	outputTargetsDir := filepath.Join(outputDir, "targets")

	// We may have other delegated roles, which we'll discover as we iterate through delegated roles in targets.json etc.
	delegatedRoles := []string{"targets"}

	for {
		if len(delegatedRoles) == 0 {
			break
		}

		roleName := delegatedRoles[0]
		if len(delegatedRoles) > 1 {
			delegatedRoles = delegatedRoles[1:]
		} else {
			delegatedRoles = []string{}
		}

		// Read the role json and copy it.
		roleFile := filepath.Join(metadataDir, fmt.Sprintf("%s.json", roleName))
		role, err := metadata.Targets().FromFile(roleFile)
		if err != nil {
			return fmt.Errorf("loading targets for role %s from %s: %w", roleName, roleFile, err)
		}

		outputRoleFile := filepath.Join(outputMetadataDir, fmt.Sprintf("%d.%s.json", role.Signed.Version, roleName))
		if err := copyFile(roleFile, outputRoleFile); err != nil {
			return err
		}

		for tfName, tfInfo := range role.Signed.Targets {
			origName := filepath.Base(tfName)
			origDir := filepath.Dir(tfName)
			origFile := filepath.Join(targetsDir, tfName)
			for _, targetHash := range tfInfo.Hashes {
				outputFile := filepath.Join(outputTargetsDir, origDir, fmt.Sprintf("%s.%s", targetHash.String(), origName))

				outputDir := filepath.Dir(outputFile)
				if err := os.MkdirAll(outputDir, 0755); err != nil {
					return fmt.Errorf("creating output directory %s: %w", outputDir, err)
				}

				if err := copyFile(origFile, outputFile); err != nil {
					return err
				}
			}
		}

		if role.Signed.Delegations != nil {
			for _, r := range role.Signed.Delegations.Roles {
				delegatedRoles = append(delegatedRoles, r.Name)
			}
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading file %s to copy: %w", src, err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return fmt.Errorf("writing file %s to %s: %w", src, dst, err)
	}

	return nil
}
