package estimator

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	oiqReleasesAPI = "https://api.github.com/repos/terrateamio/openinfraquote/releases/latest"
	oiqDownloadURL = "https://github.com/terrateamio/openinfraquote/releases/download/%s/oiq-%s-%s-%s.tar.gz"
	oiqPriceURL    = "https://oiq.terrateam.io/prices.csv.gz"
	oiqBinaryName  = "oiq"
)

// githubRelease represents the minimal fields we need from the GitHub releases API.
type githubRelease struct {
	TagName string `json:"tag_name"`
}

// progressReader wraps an io.Reader and reports download progress to the terminal.
type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	label      string
	mu         sync.Mutex
	lastPrint  time.Time
}

// newProgressReader creates a progress reader that displays a progress bar.
// If total is <= 0, it shows downloaded bytes without a percentage.
func newProgressReader(reader io.Reader, total int64, label string) *progressReader {
	return &progressReader{
		reader: reader,
		total:  total,
		label:  label,
	}
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.mu.Lock()
		pr.downloaded += int64(n)
		// Throttle output to avoid flooding the terminal
		if time.Since(pr.lastPrint) > 100*time.Millisecond || err == io.EOF {
			pr.render()
			pr.lastPrint = time.Now()
		}
		pr.mu.Unlock()
	}
	return n, err
}

func (pr *progressReader) render() {
	barWidth := 30

	if pr.total > 0 {
		pct := float64(pr.downloaded) / float64(pr.total)
		filled := int(pct * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		fmt.Fprintf(os.Stderr, "\r  %s [%s] %.1f%% (%s / %s)",
			pr.label, bar, pct*100,
			formatBytes(pr.downloaded), formatBytes(pr.total))
	} else {
		fmt.Fprintf(os.Stderr, "\r  %s %s downloaded",
			pr.label, formatBytes(pr.downloaded))
	}
}

// finish prints the final state and moves to a new line.
func (pr *progressReader) finish() {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.render()
	fmt.Fprintln(os.Stderr)
}

// formatBytes returns a human-readable byte size string.
func formatBytes(b int64) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// getOIQPlatform maps Go's runtime.GOOS and runtime.GOARCH to OIQ's naming convention.
// OIQ uses "macos" for darwin/arm64, "darwin" for darwin/amd64, and "linux" for linux.
func getOIQPlatform() (osName string, arch string, err error) {
	switch runtime.GOOS {
	case "linux":
		osName = "linux"
		switch runtime.GOARCH {
		case "amd64":
			arch = "amd64"
		case "arm64":
			arch = "arm64"
		default:
			return "", "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
		}
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			// Try macos-arm64 first, fallback to darwin-amd64 (Rosetta)
			osName = "macos"
			arch = "arm64"
		case "amd64":
			osName = "darwin"
			arch = "amd64"
		default:
			return "", "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
		}
	default:
		return "", "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	return osName, arch, nil
}

// getLatestVersion fetches the latest OIQ release version from GitHub.
func getLatestVersion() (string, error) {
	resp, err := http.Get(oiqReleasesAPI)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest OIQ release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse release response: %w", err)
	}

	if release.TagName == "" {
		return "", fmt.Errorf("no tag_name found in release response")
	}

	return release.TagName, nil
}

// downloadAndExtractBinary downloads the OIQ tar.gz archive and extracts the binary to destDir.
func downloadAndExtractBinary(url string, destDir string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download OIQ binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	progress := newProgressReader(resp.Body, resp.ContentLength, "Downloading OIQ")
	gzReader, err := gzip.NewReader(progress)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()
	defer progress.finish()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar archive: %w", err)
		}

		// We only care about the oiq binary
		name := filepath.Base(header.Name)
		if header.Typeflag == tar.TypeReg && (name == oiqBinaryName || strings.HasPrefix(name, "oiq")) {
			destPath := filepath.Join(destDir, oiqBinaryName)

			outFile, err := os.Create(destPath)
			if err != nil {
				return "", fmt.Errorf("failed to create binary file: %w", err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return "", fmt.Errorf("failed to write binary: %w", err)
			}
			outFile.Close()

			if err := os.Chmod(destPath, 0755); err != nil {
				return "", fmt.Errorf("failed to make binary executable: %w", err)
			}

			return destPath, nil
		}
	}

	return "", fmt.Errorf("oiq binary not found in archive")
}

// EnsureOIQInstalled checks if OIQ is available and installs it if not.
// It returns the path to the oiq binary.
func EnsureOIQInstalled(cacheDir string) (string, error) {
	cachedBinary := filepath.Join(cacheDir, oiqBinaryName)
	if _, err := os.Stat(cachedBinary); err == nil {
		return cachedBinary, nil
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	osName, arch, err := getOIQPlatform()
	if err != nil {
		return "", err
	}

	version, err := getLatestVersion()
	if err != nil {
		return "", err
	}

	// Try primary URL
	downloadURL := fmt.Sprintf(oiqDownloadURL, version, osName, arch, version)
	fmt.Printf("📦 Installing OpenInfraQuote %s (%s/%s)...\n", version, osName, arch)

	binaryPath, err := downloadAndExtractBinary(downloadURL, cacheDir)
	if err != nil {
		// Fallback: if macos-arm64 doesn't exist, try darwin-amd64 (runs via Rosetta)
		if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
			fmt.Println("⚠️  macOS ARM64 build not available, falling back to darwin-amd64 (Rosetta)...")
			downloadURL = fmt.Sprintf(oiqDownloadURL, version, "darwin", "amd64", version)
			binaryPath, err = downloadAndExtractBinary(downloadURL, cacheDir)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	fmt.Println("✅ OpenInfraQuote installed successfully")
	return binaryPath, nil
}

// DownloadPriceSheet downloads the latest pricing CSV to a temp file and returns the path.
// The caller is responsible for cleaning up the file.
func DownloadPriceSheet() (string, error) {
	fmt.Println("📊 Fetching latest AWS pricing data...")

	resp, err := http.Get(oiqPriceURL)
	if err != nil {
		return "", fmt.Errorf("failed to download price sheet: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("price sheet download returned status %d", resp.StatusCode)
	}

	progress := newProgressReader(resp.Body, resp.ContentLength, "Downloading prices")
	gzReader, err := gzip.NewReader(progress)
	if err != nil {
		return "", fmt.Errorf("failed to decompress price sheet: %w", err)
	}
	defer gzReader.Close()
	defer progress.finish()

	tmpFile, err := os.CreateTemp("", "oiq-prices-*.csv")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := io.Copy(tmpFile, gzReader); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write price sheet: %w", err)
	}
	tmpFile.Close()

	return tmpFile.Name(), nil
}

func RunOIQCombined(oiqPath, priceSheet string, planPaths []string, region string) (map[string][]byte, error) {
	results := make(map[string][]byte)

	for _, planPath := range planPaths {
		matchCmd := exec.Command(oiqPath, "match", "--pricesheet", priceSheet, planPath)

		priceArgs := []string{"price", "--format", "json"}
		if region != "" {
			priceArgs = append(priceArgs, "--region", region)
		}
		priceCmd := exec.Command(oiqPath, priceArgs...)

		pipe, err := matchCmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create pipe for %s: %w", planPath, err)
		}
		priceCmd.Stdin = pipe

		var priceOut bytes.Buffer
		var matchStderr, priceStderr bytes.Buffer
		matchCmd.Stderr = &matchStderr
		priceCmd.Stdout = &priceOut
		priceCmd.Stderr = &priceStderr

		if err := matchCmd.Start(); err != nil {
			return nil, fmt.Errorf("oiq match failed to start for %s: %w", planPath, err)
		}
		if err := priceCmd.Start(); err != nil {
			return nil, fmt.Errorf("oiq price failed to start for %s: %w", planPath, err)
		}
		if err := matchCmd.Wait(); err != nil {
			return nil, fmt.Errorf("oiq match failed for %s: %s: %w", planPath, matchStderr.String(), err)
		}
		if err := priceCmd.Wait(); err != nil {
			return nil, fmt.Errorf("oiq price failed for %s: %s: %w", planPath, priceStderr.String(), err)
		}

		serviceName := filepath.Base(filepath.Dir(planPath))
		results[serviceName] = priceOut.Bytes()
	}

	return results, nil
}