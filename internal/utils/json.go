package utils

import (
	"encoding/json"
	"path/filepath"
)

/*
LoadFile reads the JSON/YAML/TOML file at the given path into a typed object.
*/
func LoadFile[T any](path string) (*T, error) {
	allowedExt := map[string]struct{}{".json": {}}
	baseDir := filepath.Dir(path)
	jsonData, err := SafeReadFile(path, baseDir, allowedExt)
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

// PrettyJSON formats the given object as a pretty-printed JSON string.
func PrettyJSON[T any](object T) (string, error) {
	data, err := json.MarshalIndent(object, "", "    ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
