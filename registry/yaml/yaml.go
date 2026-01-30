package yaml

import (
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

var versionRe = regexp.MustCompile(`(.+?)-?v?([0-9]+\.[0-9]+(?:\.[0-9]+)?)?\.(yaml|yml)$`)

func ParseFile(path string) (map[string]interface{}, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var content map[string]interface{}
	if err := yaml.Unmarshal(data, content); err != nil {
		return nil, "", err
	}
	name := filepath.Base(path)
	matches := versionRe.FindStringSubmatch(name)
	ver := "unknown"
	if len(matches) > 2 && matches[2] != "" {
		ver = matches[2]
	}
	return content, ver, nil
}
