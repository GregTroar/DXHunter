package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	CtyPlistFilename = "cty.plist"
	CtyPlistZipName  = "cty.zip"
)

type UpdateResult struct {
	Success       bool      `json:"success"`
	Message       string    `json:"message"`
	EntriesBefore int       `json:"entriesBefore"`
	EntriesAfter  int       `json:"entriesAfter"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// UpdateCtyPlist télécharge cty.plist depuis country-files.com
// et recharge la base en mémoire.
func UpdateCtyPlist(ctyPath string) (*UpdateResult, error) {
	result := &UpdateResult{UpdatedAt: time.Now()}

	if ctyDB != nil {
		ctyDB.mu.RLock()
		result.EntriesBefore = len(ctyDB.entries)
		ctyDB.mu.RUnlock()
	}

	Log.Info("Downloading cty.plist update from country-files.com...")

	// 1. Télécharger (zip ou plist direct)
	data, err := downloadCtyZip()
	if err != nil {
		result.Message = fmt.Sprintf("Download failed: %v", err)
		return result, err
	}

	// 2. Si c'est un zip, extraire cty.plist ; sinon utiliser directement
	var plistData []byte
	if isZip(data) {
		plistData, err = extractCtyPlistFromZip(data)
		if err != nil {
			result.Message = fmt.Sprintf("Extraction failed: %v", err)
			return result, err
		}
		Log.Infof("Extracted cty.plist from zip: %d bytes", len(plistData))
	} else if isPlist(data) {
		plistData = data
		Log.Infof("Downloaded cty.plist directly: %d bytes", len(plistData))
	} else {
		result.Message = "Downloaded file is neither a zip nor a plist"
		return result, fmt.Errorf(result.Message)
	}

	// 3. Sauvegarder sur disque
	if err := backupAndSave(ctyPath, plistData); err != nil {
		result.Message = fmt.Sprintf("Save failed: %v", err)
		return result, err
	}

	// 4. Recharger la base en mémoire
	if err := ReloadCtyDB(ctyPath); err != nil {
		result.Message = fmt.Sprintf("Reload failed: %v", err)
		return result, err
	}

	if ctyDB != nil {
		ctyDB.mu.RLock()
		result.EntriesAfter = len(ctyDB.entries)
		ctyDB.mu.RUnlock()
	}

	result.Success = true
	result.Message = fmt.Sprintf("cty.plist updated successfully (%d → %d entries)",
		result.EntriesBefore, result.EntriesAfter)

	Log.Infof("✅ %s", result.Message)
	return result, nil
}

func downloadCtyZip() ([]byte, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// Étape 1 : trouver l'URL du dernier article Big CTY
	articleURL, err := findLatestCtyArticleURL(client)
	if err != nil {
		return nil, fmt.Errorf("cannot find latest CTY article: %w", err)
	}
	Log.Infof("Latest CTY article: %s", articleURL)

	// Étape 2 : scraper l'article pour trouver le lien de téléchargement
	downloadURL, err := findCtyDownloadURL(client, articleURL)
	if err != nil {
		return nil, fmt.Errorf("cannot find download link in article: %w", err)
	}
	Log.Infof("CTY download URL: %s", downloadURL)

	// Étape 3 : télécharger le fichier
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "FlexDXCluster/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	return data, nil
}

// findLatestCtyArticleURL scrape la home page et retourne l'URL du dernier article CTY
func findLatestCtyArticleURL(client *http.Client) (string, error) {
	req, _ := http.NewRequest("GET", "https://www.country-files.com/", nil)
	req.Header.Set("User-Agent", "FlexDXCluster/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Le premier article contient "CTY-" dans le titre, ex: "CTY-3610 – 25 February 2026"
	// On cherche le lien "Continue reading" ou le titre du premier article CTY
	re := regexp.MustCompile(`href="(https://www\.country-files\.com/cty-\d+[^"]+)"`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		return "", fmt.Errorf("no CTY article link found on home page")
	}

	return matches[1], nil
}

// findCtyDownloadURL scrape un article et retourne le lien XML (cty.plist)
func findCtyDownloadURL(client *http.Client, articleURL string) (string, error) {
	req, _ := http.NewRequest("GET", articleURL, nil)
	req.Header.Set("User-Agent", "FlexDXCluster/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	content := string(body)

	// Chercher le lien "XML" dans la ligne Download
	// Format: Download: [ CTY-3610 | CT8 | Win-Test | WriteLog | XML ]
	// Le lien XML pointe vers un fichier .plist ou .zip
	re := regexp.MustCompile(`href="([^"]+)"[^>]*>XML<`)
	matches := re.FindStringSubmatch(content)
	if len(matches) >= 2 {
		url := matches[1]
		if strings.HasPrefix(url, "/") {
			url = "https://www.country-files.com" + url
		}
		return url, nil
	}

	return "", fmt.Errorf("no XML download link found in article")
}

// extractCtyPlistFromZip extrait le fichier cty.plist depuis les données zip
func extractCtyPlistFromZip(zipData []byte) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		// Peut-être que ce n'est pas un zip mais directement le plist
		if isPlist(zipData) {
			Log.Info("Response is directly a plist file (not zipped)")
			return zipData, nil
		}
		return nil, fmt.Errorf("not a valid zip file: %w", err)
	}

	// Chercher cty.plist dans le zip
	for _, f := range r.File {
		name := strings.ToLower(filepath.Base(f.Name))
		if name == "cty.plist" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("opening %s in zip: %w", f.Name, err)
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("reading %s from zip: %w", f.Name, err)
			}

			Log.Infof("Found %s in zip (%d bytes)", f.Name, len(data))
			return data, nil
		}
	}

	// Lister les fichiers trouvés pour debug
	var names []string
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	return nil, fmt.Errorf("cty.plist not found in zip, found: %s", strings.Join(names, ", "))
}

// isZip vérifie si les données commencent par la magic bytes ZIP (PK\x03\x04)
func isZip(data []byte) bool {
	return len(data) > 4 && data[0] == 0x50 && data[1] == 0x4B && data[2] == 0x03 && data[3] == 0x04
}

// isPlist vérifie rapidement si les données ressemblent à un fichier plist
func isPlist(data []byte) bool {
	s := strings.TrimSpace(string(data[:min(200, len(data))]))
	return strings.Contains(s, "<?xml") && strings.Contains(s, "plist")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// backupAndSave sauvegarde l'ancien cty.plist et écrit le nouveau
func backupAndSave(ctyPath string, data []byte) error {
	// Backup de l'ancien fichier
	if _, err := os.Stat(ctyPath); err == nil {
		backupPath := ctyPath + ".bak"
		if err := os.Rename(ctyPath, backupPath); err != nil {
			Log.Warnf("Could not backup %s to %s: %v", ctyPath, backupPath, err)
			// Non fatal, on continue
		} else {
			Log.Infof("Backed up old cty.plist to %s", backupPath)
		}
	}

	// Écrire le nouveau fichier
	if err := os.WriteFile(ctyPath, data, 0644); err != nil {
		// Restaurer le backup si l'écriture échoue
		backupPath := ctyPath + ".bak"
		if _, berr := os.Stat(backupPath); berr == nil {
			os.Rename(backupPath, ctyPath)
			Log.Warn("Restored backup after write failure")
		}
		return fmt.Errorf("writing cty.plist: %w", err)
	}

	return nil
}
