package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	CtyPlistURL      = "https://www.country-files.com/category/big-cty/?action=download&type=xml"
	CtyPlistFilename = "cty.plist"
	CtyPlistZipName  = "cty.zip" // nom temporaire du zip téléchargé
)

type UpdateResult struct {
	Success       bool      `json:"success"`
	Message       string    `json:"message"`
	EntriesBefore int       `json:"entriesBefore"`
	EntriesAfter  int       `json:"entriesAfter"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// UpdateCtyPlist télécharge le zip depuis country-files.com,
// extrait cty.plist et recharge la base en mémoire.
func UpdateCtyPlist(ctyPath string) (*UpdateResult, error) {
	result := &UpdateResult{UpdatedAt: time.Now()}

	if ctyDB != nil {
		ctyDB.mu.RLock()
		result.EntriesBefore = len(ctyDB.entries)
		ctyDB.mu.RUnlock()
	}

	Log.Info("Downloading cty.plist update from country-files.com...")

	// 1. Télécharger le zip
	zipData, err := downloadCtyZip()
	if err != nil {
		result.Message = fmt.Sprintf("Download failed: %v", err)
		return result, err
	}
	Log.Infof("Downloaded zip: %d bytes", len(zipData))

	// 2. Extraire cty.plist du zip
	plistData, err := extractCtyPlistFromZip(zipData)
	if err != nil {
		result.Message = fmt.Sprintf("Extraction failed: %v", err)
		return result, err
	}
	Log.Infof("Extracted cty.plist: %d bytes", len(plistData))

	// 3. Sauvegarder sur disque (backup de l'ancien d'abord)
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

// downloadCtyZip télécharge le fichier zip depuis country-files.com
func downloadCtyZip() ([]byte, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// country-files.com propose le téléchargement XML (plist) via cette URL
	req, err := http.NewRequest("GET", CtyPlistURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "FlexDXCluster/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("empty response from server")
	}

	return data, nil
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
