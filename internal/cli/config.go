package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/zhurong/jianwu/internal/workspace"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read or write configuration",
	}
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigListCmd())
	return cmd
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value by dotted key (e.g. models.outline.provider)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wsRoot, err := workspace.FindWorkspace(".")
			if err != nil {
				return &InfoError{Err: err, Code: ExitCodeWorkspaceNotFound}
			}
			ws, err := workspace.Load(wsRoot)
			if err != nil {
				return err
			}
			v, err := getConfigField(ws.Config, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), v)
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value in the workspace config.yaml",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			wsRoot, err := workspace.FindWorkspace(".")
			if err != nil {
				return &InfoError{Err: err, Code: ExitCodeWorkspaceNotFound}
			}
			ws, err := workspace.Load(wsRoot)
			if err != nil {
				return err
			}
			if err := setConfigField(ws.Config, args[0], args[1]); err != nil {
				return err
			}
			// Write back to workspace config.yaml
			data, err := yaml.Marshal(ws.Config)
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}
			path := filepath.Join(wsRoot, ".jianwu", "config.yaml")
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "set %s = %s\n", args[0], args[1])
			return nil
		},
	}
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all config keys and values",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			wsRoot, err := workspace.FindWorkspace(".")
			if err != nil {
				return &InfoError{Err: err, Code: ExitCodeWorkspaceNotFound}
			}
			ws, err := workspace.Load(wsRoot)
			if err != nil {
				return err
			}
			for _, line := range flattenConfig(ws.Config) {
				fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			return nil
		},
	}
}

// getConfigField navigates a dotted path against the Config struct.
func getConfigField(cfg any, key string) (string, error) {
	v := reflect.ValueOf(cfg)
	for _, part := range strings.Split(key, ".") {
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return "", fmt.Errorf("key %q: %s is not a struct", key, v.Kind())
		}
		f := v.FieldByName(toExportedName(part))
		if !f.IsValid() {
			return "", fmt.Errorf("unknown config key %q (field %q)", key, part)
		}
		v = f
	}
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	return fmt.Sprintf("%v", v.Interface()), nil
}

// setConfigField navigates a dotted path and sets the leaf field.
func setConfigField(cfg any, key, value string) error {
	v := reflect.ValueOf(cfg).Elem()
	parts := strings.Split(key, ".")
	for i, part := range parts {
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return fmt.Errorf("key %q: not a struct at %s", key, part)
		}
		f := v.FieldByName(toExportedName(part))
		if !f.IsValid() {
			return fmt.Errorf("unknown config key %q (field %q)", key, part)
		}
		if i == len(parts)-1 {
			return assignField(f, value)
		}
		v = f
	}
	return nil
}

func assignField(f reflect.Value, value string) error {
	switch f.Kind() {
	case reflect.String:
		f.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("expected int, got %q: %w", value, err)
		}
		f.SetInt(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("expected bool, got %q: %w", value, err)
		}
		f.SetBool(b)
	default:
		return fmt.Errorf("setting field of kind %s not supported", f.Kind())
	}
	return nil
}

// toExportedName capitalizes the first letter: "outline" → "Outline".
func toExportedName(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// flattenConfig returns "key = value" lines for every leaf scalar.
func flattenConfig(cfg any) []string {
	var out []string
	var walk func(prefix string, v reflect.Value)
	walk = func(prefix string, v reflect.Value) {
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		switch v.Kind() {
		case reflect.Struct:
			t := v.Type()
			for i := 0; i < v.NumField(); i++ {
				f := v.Field(i)
				name := strings.ToLower(t.Field(i).Name)
				key := name
				if prefix != "" {
					key = prefix + "." + name
				}
				walk(key, f)
			}
		case reflect.Slice:
			// Skip slices (lists) for v0.1 list output
		default:
			if !v.IsZero() {
				out = append(out, fmt.Sprintf("%s = %v", prefix, v.Interface()))
			}
		}
	}
	walk("", reflect.ValueOf(cfg).Elem())
	return out
}
