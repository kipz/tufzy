package display

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/kipz/tufzy/internal/client"
)

var (
	green  = color.New(color.FgGreen).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	cyan   = color.New(color.FgCyan).SprintFunc()
	bold   = color.New(color.Bold).SprintFunc()
)

// ShowRepositoryHeader displays a brief repository header
func ShowRepositoryHeader(info *client.RepositoryInfo) {
	fmt.Printf("\n%s %s\n", green("✅"), bold("TUF Repository"))
	fmt.Printf("📍 Metadata: %s\n", cyan(info.MetadataURL))
	fmt.Printf("📦 Targets:  %s\n\n", cyan(info.TargetsURL))
}

// ShowTargets displays a table of available targets
func ShowTargets(targets []client.TargetInfo) {
	if len(targets) == 0 {
		fmt.Printf("%s No targets found\n", yellow("⚠️"))
		return
	}

	fmt.Printf("%s %s (%d)\n\n", bold("🎯"), bold("Targets"), len(targets))

	// Find max name length for alignment
	maxNameLen := 20
	for _, target := range targets {
		if len(target.Name) > maxNameLen {
			maxNameLen = len(target.Name)
		}
	}
	if maxNameLen > 60 {
		maxNameLen = 60
	}

	// Print header
	fmt.Printf("  %-*s  %-10s  %s\n", maxNameLen, bold("Name"), bold("Size"), bold("SHA256"))
	fmt.Printf("  %s\n", strings.Repeat("─", maxNameLen+10+16+6))

	// Print targets
	for _, target := range targets {
		name := target.Name
		if len(name) > maxNameLen {
			name = name[:maxNameLen-3] + "..."
		}

		size := formatSize(target.Length)
		hash := ""
		if sha256, ok := target.Hashes["sha256"]; ok && len(sha256) >= 16 {
			hash = sha256[:16] + "..."
		}

		fmt.Printf("  %-*s  %-10s  %s\n", maxNameLen, cyan(name), size, hash)
	}

	fmt.Println()
}

// ShowRepositoryInfo displays detailed repository information
func ShowRepositoryInfo(info *client.RepositoryInfo) {
	fmt.Printf("\n%s %s\n\n", bold("📊"), bold("Repository Information"))

	fmt.Printf("%s %s\n", bold("URLs:"), "")
	fmt.Printf("  Metadata: %s\n", cyan(info.MetadataURL))
	fmt.Printf("  Targets:  %s\n\n", cyan(info.TargetsURL))

	fmt.Printf("%s %s\n", bold("⏰"), bold("Metadata Expiry:"))
	showRoleExpiry("Root", info.RootVersion, info.RootExpires)
	showRoleExpiry("Targets", info.TargetsVersion, info.TargetsExpires)
	showRoleExpiry("Snapshot", info.SnapshotVersion, info.SnapshotExpires)
	showRoleExpiry("Timestamp", info.TimestampVersion, info.TimestampExpires)
	fmt.Println()
}

// ShowDelegations displays the delegation tree
func ShowDelegations(delegations []client.Delegation) {
	fmt.Printf("\n%s %s\n\n", bold("🌳"), bold("Delegation Tree"))

	if len(delegations) == 0 {
		fmt.Printf("  %s No delegations found\n\n", yellow("⚠️"))
		return
	}

	fmt.Println("  targets")
	for i, delegation := range delegations {
		isLast := i == len(delegations)-1
		prefix := "  ├── "
		childPrefix := "  │   "
		if isLast {
			prefix = "  └── "
			childPrefix = "      "
		}

		fmt.Printf("%s%s %s (threshold: %d/%d)\n",
			prefix,
			"📄",
			cyan(delegation.Name),
			delegation.Threshold,
			len(delegation.KeyIDs))

		if len(delegation.Paths) > 0 {
			fmt.Printf("%s└── patterns: %s\n", childPrefix, strings.Join(delegation.Paths, ", "))
		}
	}
	fmt.Println()
}

// ShowDownloadStart indicates download has started
func ShowDownloadStart(targetName, destPath string) {
	fmt.Printf("\n%s Downloading %s to %s...\n", bold("⬇️"), cyan(targetName), destPath)
}

// ShowDownloadSuccess indicates successful download
func ShowDownloadSuccess(targetName, destPath string, info *client.TargetInfo) {
	fmt.Printf("%s Downloaded and verified %s (%s)\n", green("✅"), bold(targetName), formatSize(info.Length))
	fmt.Printf("   Saved to: %s\n\n", destPath)
}

// ShowDownloadError indicates download failure
func ShowDownloadError(targetName string, err error) {
	fmt.Printf("%s Failed to download %s: %v\n\n", red("❌"), targetName, err)
}

// Helper functions

func showRoleExpiry(role string, version int64, expires time.Time) {
	now := time.Now()
	status := green("✅")
	timeUntil := ""

	if expires.Before(now) {
		status = red("❌ EXPIRED")
	} else {
		duration := time.Until(expires)
		if duration < 7*24*time.Hour {
			status = yellow("⚠️")
			timeUntil = fmt.Sprintf(" (expires in %s)", formatDuration(duration))
		} else {
			timeUntil = fmt.Sprintf(" (expires in %s)", formatDuration(duration))
		}
	}

	fmt.Printf("  %s %-10s v%-3d expires %s%s\n",
		status,
		role+":",
		version,
		expires.Format("2006-01-02"),
		timeUntil)
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days < 60 {
		return fmt.Sprintf("%d days", days)
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%d months", months)
	}
	years := months / 12
	return fmt.Sprintf("%d years", years)
}
