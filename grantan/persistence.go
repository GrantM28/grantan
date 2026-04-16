package grantan

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func saveRoomSnapshot(dataDir string, snapshot savedRoom) (string, error) {
	dir := filepath.Join(dataDir, "games")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	filePath := filepath.Join(dir, snapshot.ID+".json")
	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		return "", err
	}
	return filePath, nil
}
