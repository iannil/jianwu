package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// globalConfigPath returns the global config file location; swappable in tests.
var globalConfigPath = defaultGlobalConfigPath

func defaultGlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "jianwu", "config.yaml")
}

// SetGlobalConfigPath overrides the global config file location.
// Test seam only, mirroring SetSecretsProvider; not for production use.
func SetGlobalConfigPath(path string) {
	globalConfigPath = func() string { return path }
}

// GlobalWorkspace returns the workspace root recorded in the global config
// file (~/.config/jianwu/config.yaml `workspace:` key), or "" when unset.
// The recorded path is a start path: FindWorkspace walks up from it.
func GlobalWorkspace() (string, error) {
	path := globalConfigPath()
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read global config: %w", err)
	}
	var doc struct {
		Workspace string `yaml:"workspace"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	return strings.TrimSpace(doc.Workspace), nil
}

// SetGlobalWorkspace records the workspace root in the global config file.
// The edit is surgical (yaml.Node): other keys, unknown keys and comments
// in the file are preserved.
func SetGlobalWorkspace(root string) error {
	path := globalConfigPath()
	if path == "" {
		return fmt.Errorf("cannot resolve global config path (no home directory)")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	var doc yaml.Node
	data, err := os.ReadFile(path)
	if err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read global config: %w", err)
	}

	mapping, err := rootMappingNode(&doc)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	setMappingString(mapping, "workspace", root)

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal global config: %w", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("write global config: %w", err)
	}
	return nil
}

// rootMappingNode normalizes a parsed yaml document to its mapping node,
// creating an empty document when the file was missing or blank.
func rootMappingNode(doc *yaml.Node) (*yaml.Node, error) {
	switch {
	case doc.Kind == 0:
		mapping := &yaml.Node{Kind: yaml.MappingNode}
		doc.Kind = yaml.DocumentNode
		doc.Content = []*yaml.Node{mapping}
		return mapping, nil
	case doc.Kind == yaml.DocumentNode:
		if len(doc.Content) == 0 {
			mapping := &yaml.Node{Kind: yaml.MappingNode}
			doc.Content = []*yaml.Node{mapping}
			return mapping, nil
		}
		if doc.Content[0].Kind != yaml.MappingNode {
			return nil, fmt.Errorf("top level is not a mapping")
		}
		return doc.Content[0], nil
	case doc.Kind == yaml.MappingNode:
		return doc, nil
	default:
		return nil, fmt.Errorf("top level is not a mapping")
	}
}

// setMappingString sets key=value on a mapping node, replacing the value node
// when the key already exists.
func setMappingString(m *yaml.Node, key, value string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Value: value, Tag: "!!str"}
			return
		}
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: value, Tag: "!!str"},
	)
}
