package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
)

/*
LoadFile reads the JSON/YAML/TOML file at the given path into a typed object.
*/
func LoadFile[T any](filepath string) (*T, error) {
	jsonData, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var data T
	err = json.Unmarshal(jsonData, &data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func PrettyJSON[T any](object T) (string, error) {
	data, err := json.MarshalIndent(object, "", "    ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

/*
ListFiles returns a list of files in dirPath with the given extension.
*/
func ListFiles(dirPath string, extension string) ([]string, error) {
	var jsonFiles []string

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			if filepath.Ext(entry.Name()) == extension {
				jsonFiles = append(jsonFiles, filepath.Join(dirPath, entry.Name()))
			}
		}
	}

	return jsonFiles, nil
}
