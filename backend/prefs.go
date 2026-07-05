package backend

import (
	"encoding/json"
	"log/slog"
	"os"
	"path"
	"sync"
)

type IgnorePrefs struct {
	Text  bool `json:"text"`
	Media bool `json:"media"`
}

var (
	ignorePrefs   = make(map[int64]IgnorePrefs)
	ignorePrefsMu sync.RWMutex
)

func ignorePrefsPath() string {
	return path.Join(*BaseDataPath, "ignore.json")
}

func LoadIgnorePrefs() {
	ignorePrefsMu.Lock()
	defer ignorePrefsMu.Unlock()

	data, err := os.ReadFile(ignorePrefsPath())
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Error("Failed to read ignore prefs", "error", err)
		}
		return
	}
	if err := json.Unmarshal(data, &ignorePrefs); err != nil {
		slog.Error("Failed to parse ignore prefs", "error", err)
	}
}

func SaveIgnorePrefs() {
	ignorePrefsMu.RLock()
	data, err := json.Marshal(ignorePrefs)
	ignorePrefsMu.RUnlock()
	if err != nil {
		slog.Error("Failed to marshal ignore prefs", "error", err)
		return
	}
	if err := os.WriteFile(ignorePrefsPath(), data, 0644); err != nil {
		slog.Error("Failed to write ignore prefs", "error", err)
	}
}

func SetIgnoreText(userID int64, ignore bool) {
	ignorePrefsMu.Lock()
	p := ignorePrefs[userID]
	p.Text = ignore
	ignorePrefs[userID] = p
	ignorePrefsMu.Unlock()
	SaveIgnorePrefs()
}

func SetIgnoreMedia(userID int64, ignore bool) {
	ignorePrefsMu.Lock()
	p := ignorePrefs[userID]
	p.Media = ignore
	ignorePrefs[userID] = p
	ignorePrefsMu.Unlock()
	SaveIgnorePrefs()
}

func ShouldIgnoreText(userID int64) bool {
	ignorePrefsMu.RLock()
	defer ignorePrefsMu.RUnlock()
	return ignorePrefs[userID].Text
}

func ShouldIgnoreMedia(userID int64) bool {
	ignorePrefsMu.RLock()
	defer ignorePrefsMu.RUnlock()
	return ignorePrefs[userID].Media
}
