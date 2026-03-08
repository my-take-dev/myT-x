package cpptools

import (
	"context"
	"regexp"
	"testing"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
)

var triadDescriptionPattern = regexp.MustCompile(`^when: .+ args: .+ effect: (read|edit|exec|read or edit)\.$`)

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct cpptools command",
			command: "cpptools",
			want:    true,
		},
		{
			name:    "cpptools executable path",
			command: `C:\tools\cpptools\cpptools.exe`,
			want:    true,
		},
		{
			name:    "cpptools server executable path",
			command: `C:\tools\cpptools\cpptools-srv.exe`,
			want:    true,
		},
		{
			name:    "arg contains cpptools path",
			command: "wrapper",
			args:    []string{`C:\tools\cpptools\cpptools-srv.exe`},
			want:    true,
		},
		{
			name:    "non cpptools command",
			command: "clangd",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Matches(tt.command, tt.args)
			if got != tt.want {
				t.Fatalf("Matches(%q, %v) = %v, want %v", tt.command, tt.args, got, tt.want)
			}
		})
	}
}

func TestBuildToolsDescriptionTriadFormat(t *testing.T) {
	tools := BuildTools(&lsp.Client{}, ".")
	if len(tools) == 0 {
		t.Fatal("BuildTools returned no tools")
	}

	for _, tool := range tools {
		if !triadDescriptionPattern.MatchString(tool.Description) {
			t.Fatalf("tool %q has non-triad description: %q", tool.Name, tool.Description)
		}
	}
}

func TestHandleListMethodsIncludesGuideFields(t *testing.T) {
	svc := &service{rootDir: `C:\repo`}

	result, err := svc.handleListMethods(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("handleListMethods returned error: %v", err)
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}

	methods, ok := out["methods"].([]map[string]any)
	if !ok {
		t.Fatalf("methods type = %T, want []map[string]any", out["methods"])
	}
	if len(methods) == 0 {
		t.Fatal("methods is empty")
	}

	count, ok := out["staticMethodCount"].(int)
	if !ok {
		t.Fatalf("staticMethodCount type = %T, want int", out["staticMethodCount"])
	}
	if count != len(methods) {
		t.Fatalf("staticMethodCount=%d, want %d", count, len(methods))
	}

	for _, method := range methods {
		name, _ := method["name"].(string)
		description, _ := method["description"].(string)
		when, _ := method["when"].(string)
		args, _ := method["args"].(string)
		effect, _ := method["effect"].(string)
		source, _ := method["source"].(string)

		if name == "" || description == "" || when == "" || args == "" || effect == "" {
			t.Fatalf("method entry missing required fields: %#v", method)
		}
		if source != "static-catalog" {
			t.Fatalf("source=%q, want static-catalog", source)
		}
		if description != triadDescription(when, args, effect) {
			t.Fatalf("description=%q, want triadDescription(%q,%q,%q)", description, when, args, effect)
		}
	}
}

func TestMethodGuides(t *testing.T) {
	spec := staticMethodCatalog["cpptools/getIncludes"]
	known := guideFromSpec(spec)
	if known.Effect != "read" {
		t.Fatalf("guideFromSpec(cpptools/getIncludes).Effect=%q, want read", known.Effect)
	}
	if known.Description != triadDescription(known.When, known.Args, known.Effect) {
		t.Fatalf("guideFromSpec description is not triad: %q", known.Description)
	}

	unknown := unknownMethodGuide()
	if unknown.Effect != "exec" {
		t.Fatalf("unknownMethodGuide.Effect=%q, want exec", unknown.Effect)
	}
	if unknown.Description != triadDescription(unknown.When, unknown.Args, unknown.Effect) {
		t.Fatalf("unknownMethodGuide description is not triad: %q", unknown.Description)
	}
}
