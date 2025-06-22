/*
Package jsonutil provides utility functions for working with JSON files,
including secure file loading and pretty-printing of JSON data.
*/
package jsonutil

import (
	"encoding/json"
	"path/filepath"

	interrors "github.com/jingle2008/toolkit/internal/errors"
	fs "github.com/jingle2008/toolkit/internal/fileutil"
)

/*
LoadFile reads a JSON file at the given path into a typed object.

Parameters:
  - path: the file path to read

Returns:
  - *T: pointer to the decoded object
  - error: if the file cannot be read or decoded
*/
func LoadFile[T any](path string) (*T, error) {
	allowedExt := map[string]struct{}{".json": {}}
	baseDir := filepath.Dir(path)
	jsonData, err := fs.SafeReadFile(path, baseDir, allowedExt)
	if err != nil {
		return nil, interrors.Wrap("failed to open file", err)
	}

	var data T
	err = json.Unmarshal(jsonData, &data)
	if err != nil {
		return nil, interrors.Wrap("failed to decode JSON", err)
	}

	return &data, nil
}

/*
PrettyJSON returns a pretty-printed JSON string representation of the given object.

Parameters:
  - object: the value to marshal as JSON

Returns:
  - string: the pretty-printed JSON
  - error: if marshaling fails
*/
func PrettyJSON[T any](object T) (string, error) {
	data, err := json.MarshalIndent(object, "", "    ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
