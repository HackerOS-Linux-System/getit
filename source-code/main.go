package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ghdir [URL]",
	Short: "Błyskawiczne pobieranie folderu z GitHub",
	Long:  `ghdir - pobierz tylko wybrany folder z repozytorium GitHub bez klonowania całego projektu.`,
	Args:  cobra.ExactArgs(1),
	Run:   run,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Padding(0, 1)
	successStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	warningStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208"))
)

func run(cmd *cobra.Command, args []string) {
	rawURL := args[0]
	user, repo, branch, folder := parseGitHubURL(rawURL)
	fmt.Println(titleStyle.Render("ghdir • Pobieranie folderu z GitHub"))
	fmt.Printf(" %s/%s • %s\n", user, repo, branch)
	if folder != "" {
		fmt.Printf(" Folder: %s\n", folder)
	}
	tarURL := fmt.Sprintf("https://github.com/%s/%s/archive/refs/heads/%s.tar.gz", user, repo, branch)
	// Load cache
	cache := loadCache()
	key := fmt.Sprintf("%s/%s/%s/%s", user, repo, branch, folder)
	etag := cache[key]
	// HEAD request for checking ETag and Content-Length
	fmt.Println("\nSprawdzanie repozytorium...")
	req, err := http.NewRequest("HEAD", tarURL, nil)
	if err != nil {
		fmt.Println(errorStyle.Render("Błąd: Nie można utworzyć zapytania"))
		os.Exit(1)
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(errorStyle.Render("Błąd: Nie można pobrać repozytorium"))
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 304 {
		fmt.Println(successStyle.Render("Folder jest aktualny. Brak zmian."))
		return
	}
	if resp.StatusCode != 200 {
		fmt.Println(errorStyle.Render("Błąd: Nie można uzyskać dostępu do repozytorium"))
		os.Exit(1)
	}
	contentLength := resp.ContentLength
	useSparse := folder != "" && contentLength > 500*1024*1024 // 500 MB
	// Warn if large
	const largeThreshold = 2 * 1024 * 1024 * 1024 // 2 GB
	if contentLength > largeThreshold {
		sizeMB := contentLength / (1024 * 1024)
		fmt.Println(warningStyle.Render(fmt.Sprintf("Ostrzeżenie: Archiwum jest duże (%d MB). To może zająć dużo czasu i zużyć transfer.", sizeMB)))
		fmt.Print("Kontynuować? (y/n) ")
		var input string
		fmt.Scanln(&input)
		if strings.ToLower(strings.TrimSpace(input)) != "y" {
			fmt.Println("Przerwano.")
			return
		}
	}
	var newETag string
	var count int
	if useSparse {
		fmt.Println("\nArchiwum jest duże, przełączam na tryb git sparse-checkout dla efektywnego pobierania...")
		err = downloadWithSparse(user, repo, branch, folder)
		if err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Błąd podczas pobierania sparse: %v", err)))
			os.Exit(1)
		}
		// Count after sparse
		count = countFilesAndFolders(getLastFolder(folder))
	} else {
		// GET request for download
		fmt.Println("\nPobieranie archiwum...")
		req, err = http.NewRequest("GET", tarURL, nil)
		if err != nil {
			fmt.Println(errorStyle.Render("Błąd: Nie można utworzyć zapytania"))
			os.Exit(1)
		}
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println(errorStyle.Render("Błąd: Nie można pobrać repozytorium"))
			os.Exit(1)
		}
		if resp.StatusCode == 304 {
			fmt.Println(successStyle.Render("Folder jest aktualny. Brak zmian."))
			return
		}
		if resp.StatusCode != 200 {
			fmt.Println(errorStyle.Render("Błąd: Nie można pobrać repozytorium"))
			os.Exit(1)
		}
		newETag = resp.Header.Get("ETag")
		defer resp.Body.Close()
		bar := progressbar.DefaultBytes(
			resp.ContentLength,
			"Pobieranie",
		)
		gzr, err := gzip.NewReader(io.TeeReader(resp.Body, bar))
		if err != nil {
			fmt.Println(errorStyle.Render("Błąd: Nie można odczytać archiwum"))
			os.Exit(1)
		}
		tr := tar.NewReader(gzr)
		strip := calculateStrip(repo, branch, folder)
		fmt.Println("\nRozpakowywanie plików...")
		extractDir := "."
		atomic := folder != ""
		if atomic {
			extractDir, err = os.MkdirTemp(".", "ghdir_*")
			if err != nil {
				fmt.Println(errorStyle.Render("Błąd: Nie można utworzyć folderu tymczasowego"))
				os.Exit(1)
			}
		}
		extractStrip := strip
		if atomic {
			extractStrip++
		}
		count = 0
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				if atomic {
					os.RemoveAll(extractDir)
				}
				fmt.Println(errorStyle.Render("Błąd podczas rozpakowywania"))
				os.Exit(1)
			}
			// Zastosuj strip-components
			parts := strings.Split(header.Name, "/")
			if len(parts) <= extractStrip {
				continue
			}
			target := filepath.Join(extractDir, filepath.Join(parts[extractStrip:]...))
			if header.Typeflag == tar.TypeDir {
				os.MkdirAll(target, 0755)
				count++ // Liczymy foldery
				continue
			}
			os.MkdirAll(filepath.Dir(target), 0755)
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				if atomic {
					os.RemoveAll(extractDir)
				}
				fmt.Println(errorStyle.Render("Błąd podczas tworzenia pliku"))
				os.Exit(1)
			}
			io.Copy(f, tr)
			f.Close()
			count++
		}
		if atomic {
			targetDir := getLastFolder(folder)
			oldDir := ""
			if _, err := os.Stat(targetDir); err == nil {
				oldDir = fmt.Sprintf("%s.old.%d", targetDir, time.Now().Unix())
				err = os.Rename(targetDir, oldDir)
				if err != nil {
					os.RemoveAll(extractDir)
					fmt.Println(errorStyle.Render("Błąd podczas rename starego folderu"))
					os.Exit(1)
				}
			}
			err = os.Rename(extractDir, targetDir)
			if err != nil {
				if oldDir != "" {
					os.Rename(oldDir, targetDir)
				}
				os.RemoveAll(extractDir)
				fmt.Println(errorStyle.Render("Błąd podczas atomic rename"))
				os.Exit(1)
			}
			if oldDir != "" {
				os.RemoveAll(oldDir)
			}
		}
	}
	fmt.Println(successStyle.Render(fmt.Sprintf("\nGotowe! Pobrano %d plików/folderów", count)))
	fmt.Printf(" → %s\n", successStyle.Render("./"+getLastFolder(folder)))
	// Save new ETag to cache if archive
	if newETag != "" {
		cache[key] = newETag
		saveCache(cache)
	}
}

// Parsowanie URL-a GitHub
func parseGitHubURL(raw string) (user, repo, branch, folder string) {
	u, _ := url.Parse(raw)
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		fmt.Println(errorStyle.Render("Nieprawidłowy URL GitHub"))
		os.Exit(1)
	}
	user = parts[0]
	repo = parts[1]
	for i, p := range parts {
		if p == "tree" && len(parts) > i+1 {
			branch = parts[i+1]
			if len(parts) > i+2 {
				folder = strings.Join(parts[i+2:], "/")
			}
			return
		}
	}
	// fallback
	branch = "main"
	return
}

func calculateStrip(repo, branch, folder string) int {
	return 1 + strings.Count(folder, "/")
}

func getLastFolder(folder string) string {
	if folder == "" {
		return "."
	}
	parts := strings.Split(folder, "/")
	return parts[len(parts)-1]
}

func loadCache() map[string]string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return make(map[string]string)
	}
	dir := filepath.Join(configDir, "ghdir")
	os.MkdirAll(dir, 0755)
	file := filepath.Join(dir, "cache.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return make(map[string]string)
	}
	var cache map[string]string
	err = json.Unmarshal(data, &cache)
	if err != nil {
		return make(map[string]string)
	}
	return cache
}

func saveCache(cache map[string]string) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	dir := filepath.Join(configDir, "ghdir")
	file := filepath.Join(dir, "cache.json")
	data, err := json.MarshalIndent(cache, "", " ")
	if err != nil {
		return
	}
	os.WriteFile(file, data, 0644)
}

func downloadWithSparse(user, repo, branch, folder string) error {
	tempDir, err := os.MkdirTemp(".", "ghdir_*")
	if err != nil {
		return err
	}
	gitURL := fmt.Sprintf("https://github.com/%s/%s.git", user, repo)
	cmds := [][]string{
		{"git", "clone", "-b", branch, "--filter=blob:none", "--no-checkout", gitURL, tempDir},
		{"git", "-C", tempDir, "sparse-checkout", "init", "--cone"},
		{"git", "-C", tempDir, "sparse-checkout", "set", folder},
		{"git", "-C", tempDir, "checkout", branch},
	}
	for _, cm := range cmds {
		c := exec.Command(cm[0], cm[1:]...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			os.RemoveAll(tempDir)
			return err
		}
	}
	err = os.RemoveAll(filepath.Join(tempDir, ".git"))
	if err != nil {
		os.RemoveAll(tempDir)
		return err
	}
	// Atomic rename
	targetDir := getLastFolder(folder)
	folderPath := strings.Replace(folder, "/", string(filepath.Separator), -1)
	srcDir := filepath.Join(tempDir, folderPath)
	oldDir := ""
	if _, err := os.Stat(targetDir); err == nil {
		oldDir = fmt.Sprintf("%s.old.%d", targetDir, time.Now().Unix())
		err = os.Rename(targetDir, oldDir)
		if err != nil {
			os.RemoveAll(tempDir)
			return err
		}
	}
	err = os.Rename(srcDir, targetDir)
	if err != nil {
		if oldDir != "" {
			os.Rename(oldDir, targetDir)
		}
		os.RemoveAll(tempDir)
		return err
	}
	if oldDir != "" {
		os.RemoveAll(oldDir)
	}
	os.RemoveAll(tempDir) // Clean up empty parents
	return nil
}

func countFilesAndFolders(dir string) int {
	count := 0
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		return 0
	}
	return count - 1 // Subtract root
}

func main() {
	Execute()
}
