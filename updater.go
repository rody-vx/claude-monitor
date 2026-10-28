package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	updateRepo          = "rody-vx/claude-monitor"
	updateCheckInterval = 1 * time.Hour
	checksumsAssetName  = "checksums.txt"
)

// getUpdateBinaryPath returns the download path for a new version.
// It must not be named claude-monitor(.exe), since install kills processes by that image name on Windows.
func getUpdateBinaryPath() string {
	name := "claude-monitor-update"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(getConfigDir(), name)
}

func getReleaseAssetName() string {
	name := fmt.Sprintf("claude-monitor-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func getReleaseAssetURL(tag, name string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", updateRepo, tag, name)
}

// isRunningInstalledBinary reports whether this process runs from the binary installed by the service
func isRunningInstalledBinary() bool {
	exePath, err := getExecutablePath()
	if err != nil {
		return false
	}
	installedPath := getInstalledBinaryPath()
	if runtime.GOOS == "windows" {
		return strings.EqualFold(exePath, installedPath+".exe")
	}
	return exePath == installedPath
}

// parseVersion parses "vX.Y.Z" into its numeric parts
func parseVersion(v string) ([3]int, bool) {
	var parts [3]int
	fields := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(fields) != 3 {
		return parts, false
	}
	for i, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}

func isNewerVersion(latest, current string) bool {
	l, ok := parseVersion(latest)
	if !ok {
		return false
	}
	c, ok := parseVersion(current)
	if !ok {
		return false
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

func httpGet(url string, timeout time.Duration) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "claude-monitor/"+version)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// fetchLatestTag reads the latest release tag from the redirect of /releases/latest.
// Unlike api.github.com, this is not limited to 60 requests per hour per IP.
func fetchLatestTag() (string, error) {
	url := fmt.Sprintf("https://github.com/%s/releases/latest", updateRepo)
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "claude-monitor/"+version)

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()

	// Location: https://github.com/<repo>/releases/tag/<tag>
	location := resp.Header.Get("Location")
	const marker = "/releases/tag/"
	idx := strings.Index(location, marker)
	if idx < 0 {
		return "", fmt.Errorf("HEAD %s: HTTP %d, unexpected redirect %q", url, resp.StatusCode, location)
	}
	return location[idx+len(marker):], nil
}

// checkForUpdate returns the latest release tag and whether it is newer than the running version
func checkForUpdate() (string, bool, error) {
	tag, err := fetchLatestTag()
	if err != nil {
		return "", false, err
	}
	return tag, isNewerVersion(tag, version), nil
}

// findChecksum finds the sha256 of the given file in "sha256sum" format output
func findChecksum(sums []byte, name string) string {
	scanner := bufio.NewScanner(bytes.NewReader(sums))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0])
		}
	}
	return ""
}

// downloadUpdate downloads the binary for this platform and verifies it against checksums.txt
func downloadUpdate(tag string) (string, error) {
	assetName := getReleaseAssetName()

	sums, err := httpGet(getReleaseAssetURL(tag, checksumsAssetName), 30*time.Second)
	if err != nil {
		return "", err
	}
	expected := findChecksum(sums, assetName)
	if expected == "" {
		return "", fmt.Errorf("checksum for %s not found", assetName)
	}

	data, err := httpGet(getReleaseAssetURL(tag, assetName), 5*time.Minute)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	if actual := hex.EncodeToString(sum[:]); actual != expected {
		return "", fmt.Errorf("checksum mismatch for %s: expected %s, got %s", assetName, expected, actual)
	}

	if err := os.MkdirAll(getConfigDir(), 0700); err != nil {
		return "", err
	}
	updatePath := getUpdateBinaryPath()
	os.Remove(updatePath)
	if err := os.WriteFile(updatePath, data, 0755); err != nil {
		return "", err
	}
	return updatePath, nil
}

// tryAutoUpdate installs a newer release if available.
// The installer runs detached and restarts the service, which stops this process.
func tryAutoUpdate(logger *log.Logger) {
	tag, newer, err := checkForUpdate()
	if err != nil {
		logger.Printf("Update check error: %v", err)
		return
	}
	if !newer {
		return
	}

	logger.Printf("New version available: %s (current: %s)", tag, version)
	updatePath, err := downloadUpdate(tag)
	if err != nil {
		logger.Printf("Update download error: %v", err)
		return
	}

	if err := startDetachedInstaller(updatePath); err != nil {
		logger.Printf("Update install error: %v", err)
		return
	}
	logger.Printf("Update installer started: %s install", updatePath)
}

// cleanupUpdateBinary removes the downloaded binary left over from a previous update
func cleanupUpdateBinary() {
	os.Remove(getUpdateBinaryPath())
}
