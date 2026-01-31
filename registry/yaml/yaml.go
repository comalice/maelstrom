package yaml

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"gopkg.in/yaml.v3"
)

var versionRe = regexp.MustCompile(`(.+?)-?v?([0-9]+\.[0-9]+(?:\.[0-9]+)?)?\.(yaml|yml)$`)

func ParseFile(path string) (map[string]interface{}, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var content map[string]interface{}
	if err := yaml.Unmarshal(data, &content); err != nil {
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

func RawParseFile(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	name := filepath.Base(path)
	matches := versionRe.FindStringSubmatch(name)
	ver := "unknown"
	if len(matches) > 2 && matches[2] != "" {
		ver = matches[2]
	}
	return string(data), ver, nil
}

func Render(raw string, data any) (map[string]interface{}, error) {
	b := []byte(raw)
	if !bytes.Contains(b, []byte("{{")) {
		var content map[string]interface{}
		if err := yaml.Unmarshal(b, &content); err != nil {
			return nil, fmt.Errorf("yaml unmarshal: %w", err)
		}
		return content, nil
	}
	tmpl := template.New("yaml")
	var err error
	tmpl, err = tmpl.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("template parse: %w", err)
	}
	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template execute: %w", err)
	}
	var content map[string]interface{}
	if err = yaml.Unmarshal(buf.Bytes(), &content); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}
	return content, nil
}
