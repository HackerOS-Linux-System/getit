package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Padding(0, 1)
	successStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
)

func run(cmd *cobra.Command, args []string) {
	rawURL := args[0]

	user, repo, branch, folder := parseGitHubURL(rawURL)

	fmt.Println(titleStyle.Render("ghdir • Pobieranie folderu z GitHub"))
	fmt.Printf("   %s/%s  •  %s\n", user, repo, branch)
	if folder != "" {
		fmt.Printf("   Folder: %s\n", folder)
	}

	tarURL := fmt.Sprintf("https://github.com/%s/%s/archive/refs/heads/%s.tar.gz", user, repo, branch)

	fmt.Println("\nPobieranie archiwum...")
	resp, err := http.Get(tarURL)
	if err != nil || resp.StatusCode != 200 {
		fmt.Println(errorStyle.Render("Błąd: Nie można pobrać repozytorium"))
		os.Exit(1)
	}
	defer resp.Body.Close()

	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"Downloading",
	)

	gzr, _ := gzip.NewReader(io.TeeReader(resp.Body, bar))
	tr := tar.NewReader(gzr)

	strip := calculateStrip(repo, branch, folder)

	fmt.Println("\nRozpakowywanie plików...")

	count := 0
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(errorStyle.Render("Błąd podczas rozpakowywania"))
			os.Exit(1)
		}

		// Zastosuj strip-components
		parts := strings.Split(header.Name, "/")
		if len(parts) <= strip {
			continue
		}

		target := filepath.Join(parts[strip:]...)

		if header.Typeflag == tar.TypeDir {
			os.MkdirAll(target, 0755)
			continue
		}

		os.MkdirAll(filepath.Dir(target), 0755)
		f, _ := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		io.Copy(f, tr)
		f.Close()
		count++
	}

	fmt.Println(successStyle.Render(fmt.Sprintf("\nDone! Pobrano %d plików/folderów", count)))
	fmt.Printf("   → %s\n", successStyle.Render("./"+getLastFolder(folder)))
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
	prefix := repo + "-" + branch
	if folder != "" {
		prefix += "/" + folder
	}
	return strings.Count(prefix, "/")
}

func getLastFolder(folder string) string {
	if folder == "" {
		return "."
	}
	parts := strings.Split(folder, "/")
	return parts[len(parts)-1]
}
