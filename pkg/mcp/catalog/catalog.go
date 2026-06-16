package catalog

import "strings"

type ToolDefinition struct {
	Name         string
	DisplayName  string
	Description  string
	Category     string
	Source       string
	InputSchema  map[string]any
	PricePerCall float64
	PriceUnit    string
	FreeQuota    int
	IsRemote     bool
	SortOrder    int
}

const (
	SourceBuiltin = "builtin"

	CategoryFile    = "file"
	CategorySearch  = "search"
	CategoryShell   = "shell"
	CategoryGit     = "git"
	CategoryProject = "project"
	CategorySystem  = "system"

	PriceUnitPerCall = "per_call"
)

func BuiltinTools() []ToolDefinition {
	tools := []ToolDefinition{
		{
			Name:        "remote_read",
			DisplayName: "远程读取文件",
			Description: "读取用户已连接本地工作区中的文件。",
			Category:    CategoryFile,
			InputSchema: objectSchema([]string{"file_path"}, map[string]any{
				"file_path": stringProp("绝对路径，或相对已配置项目根目录的路径。"),
				"offset":    integerProp("可选，从第几行开始读取，按 1 开始计数。"),
				"limit":     integerProp("可选，最多读取的行数。"),
			}),
		},
		{
			Name:        "remote_write",
			DisplayName: "远程写入文件",
			Description: "向用户已连接本地工作区中的文件写入完整内容。",
			Category:    CategoryFile,
			InputSchema: objectSchema([]string{"file_path", "content"}, map[string]any{
				"file_path": stringProp("绝对路径，或相对已配置项目根目录的路径。"),
				"content":   stringProp("要写入文件的完整内容。"),
			}),
		},
		{
			Name:        "remote_edit",
			DisplayName: "远程编辑文件",
			Description: "替换用户已连接本地工作区文件中的文本。",
			Category:    CategoryFile,
			InputSchema: objectSchema([]string{"file_path", "old_string", "new_string"}, map[string]any{
				"file_path":  stringProp("绝对路径，或相对已配置项目根目录的路径。"),
				"old_string": stringProp("要替换的文本，必须精确匹配。"),
				"new_string": stringProp("替换后的文本。"),
			}),
		},
		{
			Name:        "remote_grep",
			DisplayName: "远程内容搜索",
			Description: "在用户已连接本地工作区中搜索文件内容。",
			Category:    CategorySearch,
			InputSchema: objectSchema([]string{"pattern"}, map[string]any{
				"pattern": stringProp("搜索模式。"),
				"path":    stringProp("可选，要搜索的目录或文件路径。"),
				"glob":    stringProp("可选，文件 glob 过滤条件。"),
			}),
		},
		{
			Name:        "remote_glob",
			DisplayName: "远程文件匹配",
			Description: "按 glob 模式在用户已连接本地工作区中查找文件。",
			Category:    CategorySearch,
			InputSchema: objectSchema([]string{"pattern"}, map[string]any{
				"pattern": stringProp("Glob 模式，例如 **/*.go。"),
				"path":    stringProp("可选，要搜索的目录。"),
			}),
		},
		{
			Name:        "remote_tree",
			DisplayName: "远程目录树",
			Description: "列出用户已连接本地工作区中的目录树。",
			Category:    CategoryFile,
			InputSchema: objectSchema([]string{"path"}, map[string]any{
				"path":  stringProp("目录路径。"),
				"depth": integerProp("可选，最大遍历深度。"),
			}),
		},
		{
			Name:        "remote_exec",
			DisplayName: "远程执行命令",
			Description: "在用户已连接本地工作区中执行一次性 Shell 命令。",
			Category:    CategoryShell,
			InputSchema: objectSchema([]string{"command"}, map[string]any{
				"command":    stringProp("要执行的命令。"),
				"workdir":    stringProp("可选，工作目录。"),
				"timeout_ms": integerProp("可选，超时时间，单位为毫秒。"),
			}),
		},
		{
			Name:        "remote_shell_open",
			DisplayName: "打开远程 Shell",
			Description: "在用户已连接本地工作区中打开持久 Shell 会话。",
			Category:    CategoryShell,
			InputSchema: objectSchema(nil, map[string]any{
				"shell":   stringProp("可选，Shell 路径。"),
				"workdir": stringProp("可选，工作目录。"),
			}),
		},
		{
			Name:        "remote_shell_eval",
			DisplayName: "写入远程 Shell",
			Description: "向持久 Shell 会话发送输入。",
			Category:    CategoryShell,
			InputSchema: objectSchema([]string{"session_id", "input"}, map[string]any{
				"session_id": stringProp("持久 Shell 会话 ID。"),
				"input":      stringProp("要发送给 Shell 的输入。"),
			}),
		},
		{
			Name:        "remote_git_status",
			DisplayName: "远程 Git 状态",
			Description: "获取用户已连接本地工作区的 Git 状态。",
			Category:    CategoryGit,
			InputSchema: objectSchema(nil, map[string]any{
				"workdir": stringProp("可选，代码仓库目录。"),
			}),
		},
		{
			Name:        "remote_git_diff",
			DisplayName: "远程 Git 差异",
			Description: "获取用户已连接本地工作区的 Git diff。",
			Category:    CategoryGit,
			InputSchema: objectSchema(nil, map[string]any{
				"workdir": stringProp("可选，代码仓库目录。"),
				"cached":  booleanProp("是否显示已暂存的变更。"),
			}),
		},
		{
			Name:        "remote_git_log",
			DisplayName: "远程 Git 日志",
			Description: "获取用户已连接本地工作区的近期 Git 提交。",
			Category:    CategoryGit,
			InputSchema: objectSchema(nil, map[string]any{
				"workdir": stringProp("可选，代码仓库目录。"),
				"limit":   integerProp("可选，最多返回的提交数量。"),
			}),
		},
		{
			Name:        "remote_project_info",
			DisplayName: "远程项目信息",
			Description: "汇总用户已连接本地工作区的项目元数据。",
			Category:    CategoryProject,
			InputSchema: objectSchema(nil, map[string]any{
				"path": stringProp("可选，项目目录。"),
			}),
		},
		{
			Name:        "remote_get_related_files",
			DisplayName: "远程相关文件",
			Description: "在用户已连接本地工作区中查找与指定路径相关的文件。",
			Category:    CategoryProject,
			InputSchema: objectSchema([]string{"file_path"}, map[string]any{
				"file_path": stringProp("参考文件路径。"),
			}),
		},
		{
			Name:        "remote_run_tests",
			DisplayName: "远程运行测试",
			Description: "在用户已连接本地工作区中运行项目测试命令。",
			Category:    CategoryProject,
			InputSchema: objectSchema(nil, map[string]any{
				"workdir": stringProp("可选，项目目录。"),
				"command": stringProp("可选，覆盖默认测试命令。"),
			}),
		},
		{
			Name:        "remote_install_package",
			DisplayName: "远程安装依赖",
			Description: "在用户已连接本地工作区中安装依赖包。",
			Category:    CategorySystem,
			InputSchema: objectSchema([]string{"manager", "package"}, map[string]any{
				"manager": stringProp("包管理器，例如 npm、bun、pip、go、cargo。"),
				"package": stringProp("依赖包名称。"),
				"workdir": stringProp("可选，项目目录。"),
			}),
		},
		{
			Name:        "remote_env_info",
			DisplayName: "远程环境信息",
			Description: "获取用户已连接本地机器的环境信息。",
			Category:    CategorySystem,
			InputSchema: objectSchema(nil, map[string]any{}),
		},
		{
			Name:        "server_time",
			DisplayName: "服务器时间",
			Description: "返回 data-proxy 服务器时间，包含 Unix 与 RFC3339 格式。",
			Category:    CategorySystem,
			InputSchema: objectSchema(nil, map[string]any{
				"timezone": stringProp("可选，IANA 时区名称，例如 Asia/Shanghai 或 UTC。"),
			}),
		},
		{
			Name:        "json_pretty",
			DisplayName: "JSON 格式化",
			Description: "在 data-proxy 服务器上校验并格式化 JSON 字符串。",
			Category:    CategoryProject,
			InputSchema: objectSchema([]string{"json"}, map[string]any{
				"json":   stringProp("要校验并格式化的 JSON 字符串。"),
				"indent": integerProp("可选，缩进空格数，默认 2，范围 0-8。"),
			}),
		},
	}

	for i := range tools {
		tools[i].Source = SourceBuiltin
		tools[i].PriceUnit = PriceUnitPerCall
		tools[i].IsRemote = strings.HasPrefix(tools[i].Name, "remote_")
		tools[i].SortOrder = (i + 1) * 10
	}
	return tools
}

func objectSchema(required []string, properties map[string]any) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringProp(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
	}
}

func integerProp(description string) map[string]any {
	return map[string]any{
		"type":        "integer",
		"description": description,
	}
}

func booleanProp(description string) map[string]any {
	return map[string]any{
		"type":        "boolean",
		"description": description,
	}
}
