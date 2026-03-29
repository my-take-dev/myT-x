package tools

// --- JSON スキーマビルダー（各ツールの InputSchema 定義） ---

func emptySchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func fileOnlySchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"root": map[string]any{
				"type":        "string",
				"description": "Optional root directory override.",
			},
			"relativePath": map[string]any{
				"type":        "string",
				"description": "Target file path relative to root (or absolute path).",
			},
		},
		"required": []string{"relativePath"},
	}
}

func filePositionSchema(includeCharacterRequired bool) map[string]any {
	required := []string{"relativePath"}
	if includeCharacterRequired {
		required = append(required, "line", "character")
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"root": map[string]any{
				"type":        "string",
				"description": "Optional root directory override.",
			},
			"relativePath": map[string]any{
				"type":        "string",
				"description": "Target file path relative to root (or absolute path).",
			},
			"line": map[string]any{
				"anyOf": []map[string]any{
					{"type": "integer"},
					{"type": "string"},
				},
				"description": "Line number (1-based) or a line snippet to search.",
			},
			"character": map[string]any{
				"type":        "integer",
				"description": "Character offset in UTF-16 (0-based).",
			},
			"column": map[string]any{
				"type":        "integer",
				"description": "Alias for character.",
			},
			"textTarget": map[string]any{
				"type":        "string",
				"description": "Optional text to locate symbol when character is omitted.",
			},
		},
		"required": required,
	}
}

func filePositionWithContextSchema() map[string]any {
	schema := filePositionSchema(false)
	props := schema["properties"].(map[string]any)
	props["before"] = map[string]any{
		"type":        "integer",
		"description": "Preview lines before the definition line.",
	}
	props["after"] = map[string]any{
		"type":        "integer",
		"description": "Preview lines after the definition line.",
	}
	return schema
}

func referencesSchema() map[string]any {
	schema := filePositionWithContextSchema()
	props := schema["properties"].(map[string]any)
	props["includeDeclaration"] = map[string]any{
		"type":        "boolean",
		"description": "Include declaration locations in results.",
	}
	return schema
}

func workspaceSymbolSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Workspace symbol query.",
			},
		},
		"required": []string{"query"},
	}
}

func resolveWorkspaceSymbolSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"symbol": map[string]any{
				"type":        "object",
				"description": "Workspace symbol object returned by workspace/symbol.",
			},
		},
		"required": []string{"symbol"},
	}
}

func completionSchema() map[string]any {
	schema := filePositionSchema(false)
	props := schema["properties"].(map[string]any)
	props["maxItems"] = map[string]any{
		"type":        "integer",
		"description": "Max completion items to return.",
	}
	return schema
}

func resolveCompletionItemSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"item": map[string]any{
				"type":        "object",
				"description": "Completion item object returned by textDocument/completion.",
			},
		},
		"required": []string{"item"},
	}
}

func diagnosticsSchema() map[string]any {
	schema := fileOnlySchema()
	props := schema["properties"].(map[string]any)
	props["waitMs"] = map[string]any{
		"type":        "integer",
		"description": "Wait time for push diagnostics fallback.",
	}
	props["usePull"] = map[string]any{
		"type":        "boolean",
		"description": "Try pull diagnostics first when supported.",
	}
	return schema
}

func workspaceDiagnosticsSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"identifier": map[string]any{
				"type":        "string",
				"description": "Optional identifier for workspace diagnostics request.",
			},
			"previousResultIds": map[string]any{
				"type":        "array",
				"description": "Optional previous result ids for incremental workspace diagnostics.",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"uri":   map[string]any{"type": "string", "description": "Document URI."},
						"value": map[string]any{"type": "string", "description": "Previous result id."},
					},
				},
			},
		},
	}
}

func codeActionSchema() map[string]any {
	schema := filePositionSchema(false)
	props := schema["properties"].(map[string]any)
	props["endLine"] = map[string]any{
		"type":        "integer",
		"description": "End line (1-based). Defaults to start line.",
	}
	props["endCharacter"] = map[string]any{
		"type":        "integer",
		"description": "End character (0-based UTF-16). Defaults to start character.",
	}
	return schema
}

func resolveCodeActionSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "object",
				"description": "Code action object returned by textDocument/codeAction.",
			},
		},
		"required": []string{"action"},
	}
}

func formattingSchema() map[string]any {
	schema := fileOnlySchema()
	props := schema["properties"].(map[string]any)
	props["tabSize"] = map[string]any{
		"type":        "integer",
		"description": "Tab size for formatter options.",
	}
	props["insertSpaces"] = map[string]any{
		"type":        "boolean",
		"description": "Formatter insertSpaces option.",
	}
	props["applyEdits"] = map[string]any{
		"type":        "boolean",
		"description": "Apply edits to file when true.",
	}
	return schema
}

func rangeFormattingSchema() map[string]any {
	schema := filePositionSchema(false)
	props := schema["properties"].(map[string]any)
	props["endLine"] = map[string]any{
		"type":        "integer",
		"description": "End line (1-based). Defaults to start line.",
	}
	props["endCharacter"] = map[string]any{
		"type":        "integer",
		"description": "End character (0-based UTF-16). Defaults to start character.",
	}
	props["tabSize"] = map[string]any{
		"type":        "integer",
		"description": "Tab size for formatter options.",
	}
	props["insertSpaces"] = map[string]any{
		"type":        "boolean",
		"description": "Formatter insertSpaces option.",
	}
	props["applyEdits"] = map[string]any{
		"type":        "boolean",
		"description": "Apply edits to file when true.",
	}
	return schema
}

func onTypeFormattingSchema() map[string]any {
	schema := filePositionSchema(false)
	props := schema["properties"].(map[string]any)
	props["ch"] = map[string]any{
		"type":        "string",
		"description": "Typed character that triggered formatting.",
	}
	props["tabSize"] = map[string]any{
		"type":        "integer",
		"description": "Tab size for formatter options.",
	}
	props["insertSpaces"] = map[string]any{
		"type":        "boolean",
		"description": "Formatter insertSpaces option.",
	}
	props["applyEdits"] = map[string]any{
		"type":        "boolean",
		"description": "Apply edits to file when true.",
	}
	schema["required"] = []string{"relativePath", "ch"}
	return schema
}

func renameSchema() map[string]any {
	schema := filePositionSchema(false)
	props := schema["properties"].(map[string]any)
	props["newName"] = map[string]any{
		"type":        "string",
		"description": "New symbol name.",
	}
	props["applyEdits"] = map[string]any{
		"type":        "boolean",
		"description": "Apply returned workspace edit to files when true.",
	}
	schema["required"] = []string{"relativePath", "newName"}
	return schema
}

func executeCommandSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Command name for workspace/executeCommand.",
			},
			"arguments": map[string]any{
				"type":        "array",
				"description": "Arguments for workspace/executeCommand.",
				"items":       map[string]any{},
			},
		},
		"required": []string{"command"},
	}
}
