package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const githubReleasesURL = "https://api.github.com/repos/runelock-io/rune/releases/latest"

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the Rune CLI to the latest release",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Current version: %s\n", c(accent, Version))
		fmt.Print("Checking for updates... ")

		release, err := fetchLatestRelease()
		if err != nil {
			fmt.Println()
			Error("Failed to check for updates: " + err.Error())
			return
		}

		fmt.Println(c(ok, "done"))

		if release.TagName == Version {
			Success("Already up to date (" + Version + ")")
			return
		}

		fmt.Printf("New version available: %s\n", c(accent, release.TagName))

		assetName := assetNameForPlatform()
		url := findAssetURL(release.Assets, assetName)
		if url == "" {
			Error(fmt.Sprintf("No binary found for your platform (%s/%s)", runtime.GOOS, runtime.GOARCH))
			return
		}

		execPath, err := os.Executable()
		if err != nil {
			Error("Could not determine executable path: " + err.Error())
			return
		}

		fmt.Printf("Downloading %s... ", c(accent, assetName))
		if err := downloadAndReplace(url, execPath); err != nil {
			fmt.Println()
			Error("Update failed: " + err.Error())
			return
		}
		fmt.Println(c(ok, "done"))

		Success(fmt.Sprintf("Updated to %s — restart your shell if needed", release.TagName))
	},
}

func fetchLatestRelease() (*ghRelease, error) {
	req, err := http.NewRequest("GET", githubReleasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func assetNameForPlatform() string {
	name := fmt.Sprintf("rune_%s_%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func findAssetURL(assets []ghAsset, name string) string {
	for _, a := range assets {
		if strings.EqualFold(a.Name, name) {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

func downloadAndReplace(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	// Write to a temp file alongside the destination
	tmp, err := os.CreateTemp("", "rune-update-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return err
	}

	// Atomic replace
	return os.Rename(tmpPath, destPath)
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
