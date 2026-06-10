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
			DisplayName: "Remote Read",
			Description: "Read a file from the user's connected local workspace.",
			Category:    CategoryFile,
			InputSchema: objectSchema([]string{"file_path"}, map[string]any{
				"file_path": stringProp("Absolute path or path relative to the configured project root."),
				"offset":    integerProp("Optional 1-based starting line."),
				"limit":     integerProp("Optional maximum number of lines to read."),
			}),
		},
		{
			Name:        "remote_write",
			DisplayName: "Remote Write",
			Description: "Write full content to a file in the user's connected local workspace.",
			Category:    CategoryFile,
			InputSchema: objectSchema([]string{"file_path", "content"}, map[string]any{
				"file_path": stringProp("Absolute path or path relative to the configured project root."),
				"content":   stringProp("Full file content to write."),
			}),
		},
		{
			Name:        "remote_edit",
			DisplayName: "Remote Edit",
			Description: "Replace text in a file in the user's connected local workspace.",
			Category:    CategoryFile,
			InputSchema: objectSchema([]string{"file_path", "old_string", "new_string"}, map[string]any{
				"file_path":  stringProp("Absolute path or path relative to the configured project root."),
				"old_string": stringProp("Text to replace. It must match exactly."),
				"new_string": stringProp("Replacement text."),
			}),
		},
		{
			Name:        "remote_grep",
			DisplayName: "Remote Grep",
			Description: "Search file contents in the user's connected local workspace.",
			Category:    CategorySearch,
			InputSchema: objectSchema([]string{"pattern"}, map[string]any{
				"pattern": stringProp("Search pattern."),
				"path":    stringProp("Optional directory or file path to search under."),
				"glob":    stringProp("Optional file glob filter."),
			}),
		},
		{
			Name:        "remote_glob",
			DisplayName: "Remote Glob",
			Description: "Find files by glob pattern in the user's connected local workspace.",
			Category:    CategorySearch,
			InputSchema: objectSchema([]string{"pattern"}, map[string]any{
				"pattern": stringProp("Glob pattern, for example **/*.go."),
				"path":    stringProp("Optional directory to search under."),
			}),
		},
		{
			Name:        "remote_tree",
			DisplayName: "Remote Tree",
			Description: "List a directory tree in the user's connected local workspace.",
			Category:    CategoryFile,
			InputSchema: objectSchema([]string{"path"}, map[string]any{
				"path":  stringProp("Directory path."),
				"depth": integerProp("Optional maximum depth."),
			}),
		},
		{
			Name:        "remote_exec",
			DisplayName: "Remote Exec",
			Description: "Run a one-shot shell command in the user's connected local workspace.",
			Category:    CategoryShell,
			InputSchema: objectSchema([]string{"command"}, map[string]any{
				"command":    stringProp("Command to execute."),
				"workdir":    stringProp("Optional working directory."),
				"timeout_ms": integerProp("Optional timeout in milliseconds."),
			}),
		},
		{
			Name:        "remote_shell_open",
			DisplayName: "Remote Shell Open",
			Description: "Open a persistent shell session in the user's connected local workspace.",
			Category:    CategoryShell,
			InputSchema: objectSchema(nil, map[string]any{
				"shell":   stringProp("Optional shell path."),
				"workdir": stringProp("Optional working directory."),
			}),
		},
		{
			Name:        "remote_shell_eval",
			DisplayName: "Remote Shell Eval",
			Description: "Send input to a persistent shell session.",
			Category:    CategoryShell,
			InputSchema: objectSchema([]string{"session_id", "input"}, map[string]any{
				"session_id": stringProp("Persistent shell session ID."),
				"input":      stringProp("Input to send to the shell."),
			}),
		},
		{
			Name:        "remote_git_status",
			DisplayName: "Remote Git Status",
			Description: "Get git status for the user's connected local workspace.",
			Category:    CategoryGit,
			InputSchema: objectSchema(nil, map[string]any{
				"workdir": stringProp("Optional repository directory."),
			}),
		},
		{
			Name:        "remote_git_diff",
			DisplayName: "Remote Git Diff",
			Description: "Get git diff for the user's connected local workspace.",
			Category:    CategoryGit,
			InputSchema: objectSchema(nil, map[string]any{
				"workdir": stringProp("Optional repository directory."),
				"cached":  booleanProp("Whether to show staged changes."),
			}),
		},
		{
			Name:        "remote_git_log",
			DisplayName: "Remote Git Log",
			Description: "Get recent git commits for the user's connected local workspace.",
			Category:    CategoryGit,
			InputSchema: objectSchema(nil, map[string]any{
				"workdir": stringProp("Optional repository directory."),
				"limit":   integerProp("Optional maximum number of commits."),
			}),
		},
		{
			Name:        "remote_project_info",
			DisplayName: "Remote Project Info",
			Description: "Summarize project metadata from the user's connected local workspace.",
			Category:    CategoryProject,
			InputSchema: objectSchema(nil, map[string]any{
				"path": stringProp("Optional project directory."),
			}),
		},
		{
			Name:        "remote_get_related_files",
			DisplayName: "Remote Related Files",
			Description: "Find files related to a path in the user's connected local workspace.",
			Category:    CategoryProject,
			InputSchema: objectSchema([]string{"file_path"}, map[string]any{
				"file_path": stringProp("Reference file path."),
			}),
		},
		{
			Name:        "remote_run_tests",
			DisplayName: "Remote Run Tests",
			Description: "Run the project's test command in the user's connected local workspace.",
			Category:    CategoryProject,
			InputSchema: objectSchema(nil, map[string]any{
				"workdir": stringProp("Optional project directory."),
				"command": stringProp("Optional test command override."),
			}),
		},
		{
			Name:        "remote_install_package",
			DisplayName: "Remote Install Package",
			Description: "Install a package in the user's connected local workspace.",
			Category:    CategorySystem,
			InputSchema: objectSchema([]string{"manager", "package"}, map[string]any{
				"manager": stringProp("Package manager, for example npm, bun, pip, go, cargo."),
				"package": stringProp("Package name."),
				"workdir": stringProp("Optional project directory."),
			}),
		},
		{
			Name:        "remote_env_info",
			DisplayName: "Remote Env Info",
			Description: "Get environment information from the user's connected local machine.",
			Category:    CategorySystem,
			InputSchema: objectSchema(nil, map[string]any{}),
		},
		{
			Name:        "server_time",
			DisplayName: "Server Time",
			Description: "Return the data-proxy server time in Unix and RFC3339 formats.",
			Category:    CategorySystem,
			InputSchema: objectSchema(nil, map[string]any{
				"timezone": stringProp("Optional IANA timezone name, for example Asia/Shanghai or UTC."),
			}),
		},
		{
			Name:        "json_pretty",
			DisplayName: "JSON Pretty",
			Description: "Validate and format a JSON string on the data-proxy server.",
			Category:    CategoryProject,
			InputSchema: objectSchema([]string{"json"}, map[string]any{
				"json":   stringProp("JSON string to validate and format."),
				"indent": integerProp("Optional indentation spaces, default 2, range 0-8."),
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
