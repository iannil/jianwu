package workspace

import (
	"os"
	"strings"

	"github.com/iannil/jianwu/internal/config"
)

// Workspace source identifiers returned by ResolveRoot.
const (
	SourceFlag   = "flag"   // CLI --dir
	SourceEnv    = "env"    // JIANWU_WORKSPACE
	SourceConfig = "config" // global config.yaml `workspace:` key
	SourceCWD    = "cwd"    // fallback: current directory
)

// EnvWorkspace is the environment variable that configures the workspace root.
const EnvWorkspace = "JIANWU_WORKSPACE"

// ResolveRoot returns the workspace start path by precedence:
// explicit flag > JIANWU_WORKSPACE env > global config `workspace` key > CWD.
// The returned path is a START path: FindWorkspace walks up from it looking
// for the .jianwu marker, so it may point at the workspace itself or any
// ancestor directory. The second return names the source.
func ResolveRoot(flagPath string) (string, string) {
	if strings.TrimSpace(flagPath) != "" {
		return flagPath, SourceFlag
	}
	if env := strings.TrimSpace(os.Getenv(EnvWorkspace)); env != "" {
		return env, SourceEnv
	}
	if ws, err := config.GlobalWorkspace(); err == nil && ws != "" {
		return ws, SourceConfig
	}
	return ".", SourceCWD
}

// SourceZh returns a human-readable label for a workspace source.
func SourceZh(source string) string {
	switch source {
	case SourceFlag:
		return "命令行参数 --dir"
	case SourceEnv:
		return "环境变量 " + EnvWorkspace
	case SourceConfig:
		return "全局配置文件 workspace 键"
	case SourceCWD:
		return "当前目录"
	default:
		return source
	}
}
