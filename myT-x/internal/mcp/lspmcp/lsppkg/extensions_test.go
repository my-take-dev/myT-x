package lsppkg

import (
	"strings"
	"testing"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
)

func TestBuildToolsMatchesGopls(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("gopls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected gopls tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "gopls_execute_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected gopls_execute_command in tools")
	}
}

func TestBuildToolsMatchesABAP(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("abaplint", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected abap tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "abap_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected abap_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesAS2(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("as2-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected as2 tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "as2_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected as2_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesASN1(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("titan-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected asn1 tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "asn1_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected asn1_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesAda(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ada_language_server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected ada tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "ada_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected ada_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesAgda(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("agda-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected agda tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "agda_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected agda_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesAML(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("aml-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected aml tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "aml_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected aml_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesAnsible(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ansible-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected ansible tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "ansible_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected ansible_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesAngular(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ngserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected angular tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "angular_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected angular_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesAntlr(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("antlr-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected antlr tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "antlr_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected antlr_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesAPIElements(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("apielements-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected apielements tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "apielements_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected apielements_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesAPL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("apl-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected apl tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "apl_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected apl_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCamel(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("camel-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected camel tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "camel_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected camel_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesApacheDispatcher(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("apache-dispatcher-config-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected apachedispatcher tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "apachedispatcher_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected apachedispatcher_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesApex(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("apex-jorje-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected apex tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "apex_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected apex_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesAstro(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("astro-ls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected astro tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "astro_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected astro_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesAWK(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("awk-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected awk tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "awk_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected awk_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesBake(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("docker-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected bake tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "bake_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected bake_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesBallerina(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ballerina-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected ballerina tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "ballerina_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected ballerina_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesBash(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("bash-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected bash tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "bash_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected bash_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesBatch(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("rech-editor-batch", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected batch tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "batch_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected batch_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesBazel(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("bazel-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected bazel tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "bazel_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected bazel_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesBicep(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("bicep-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected bicep tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "bicep_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected bicep_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesBitBake(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("bitbake-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected bitbake tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "bitbake_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected bitbake_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesBSL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("bsl-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected bsl tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "bsl_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected bsl_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesBoriel(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("boriel-basic-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected boriel tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "boriel_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected boriel_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesBProB(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("b-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected bprob tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "bprob_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected bprob_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesBrighterScript(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("brighterscript-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected brighterscript tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "brighterscript_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected brighterscript_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCaddy(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("caddyfile-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected caddy tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "caddy_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected caddy_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCDS(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("cds-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected cds tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "cds_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected cds_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCSSLS(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("vscode-css-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected cssls tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "cssls_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected cssls_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCeylon(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ceylon-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected ceylon tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "ceylon_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected ceylon_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesClarity(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("clarity-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected clarity tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "clarity_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected clarity_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesClojure(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("clojure-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected clojure tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "clojure_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected clojure_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCMake(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("cmake-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected cmake tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "cmake_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected cmake_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCommonLisp(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("cl-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected commonlisp tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "commonlisp_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected commonlisp_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesChapel(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("chapel-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected chapel tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "chapel_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected chapel_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCoq(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("coq-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected coq tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "coq_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected coq_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCobol(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("cobol-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected cobol tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "cobol_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected cobol_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCodeQL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("codeql-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected codeql tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "codeql_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected codeql_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCoffeeScript(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("coffeesense", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected coffeescript tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "coffeescript_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected coffeescript_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesClangd(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("clangd", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected clangd tools, got none")
	}

	foundSwitch := false
	for _, tool := range tools {
		if tool.Name == "clangd_switch_source_header" {
			foundSwitch = true
			break
		}
	}
	if !foundSwitch {
		t.Fatalf("expected clangd_switch_source_header in tools")
	}
}

func TestBuildToolsMatchesCcls(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ccls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected ccls tools, got none")
	}

	foundCall := false
	for _, tool := range tools {
		if tool.Name == "ccls_get_call_hierarchy" {
			foundCall = true
			break
		}
	}
	if !foundCall {
		t.Fatalf("expected ccls_get_call_hierarchy in tools")
	}
}

func TestBuildToolsMatchesCSharp(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("omnisharp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected csharp tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "csharp_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected csharp_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesQmlls(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("qmlls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected qmlls tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "qmlls_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected qmlls_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesHLASM(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("hlasm-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected hlasm tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "hlasm_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected hlasm_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesIBMi(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ibmi-languages", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected ibmi tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "ibmi_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected ibmi_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCquery(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("cquery", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected cquery tools, got none")
	}

	foundCallers := false
	for _, tool := range tools {
		if tool.Name == "cquery_get_callers" {
			foundCallers = true
			break
		}
	}
	if !foundCallers {
		t.Fatalf("expected cquery_get_callers in tools")
	}
}

func TestBuildToolsMatchesCrystal(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("crystalline", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected crystal tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "crystal_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected crystal_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCWL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("cwl-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected cwl tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "cwl_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected cwl_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCucumber(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("cucumber-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected cucumber tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "cucumber_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected cucumber_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCython(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("cyright-langserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected cython tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "cython_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected cython_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesDLang(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("serve-d", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected dlang tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "dlang_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected dlang_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesDart(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("dart-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected dart tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "dart_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected dart_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesDataPack(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("datapack-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected datapack tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "datapack_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected datapack_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesDebian(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("debputy-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected debian tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "debian_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected debian_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesDelphi(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("delphilsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected delphi tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "delphi_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected delphi_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesDenizenScript(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("denizen-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected denizenscript tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "denizenscript_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected denizenscript_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesDevicetree(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("dts-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected devicetree tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "devicetree_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected devicetree_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesDeno(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("denols", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected deno tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "deno_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected deno_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesDockerfile(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("docker-langserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected dockerfile tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "dockerfile_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected dockerfile_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesDreamMaker(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("dm-langserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected dreammaker tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "dreammaker_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected dreammaker_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesEgglog(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("egglog-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected egglog tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "egglog_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected egglog_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesEmacsLisp(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ellsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected emacslisp tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "emacslisp_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected emacslisp_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesErlang(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("erlang_ls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected erlang tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "erlang_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected erlang_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesErg(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("els", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected erg tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "erg_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected erg_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesElixir(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("elixir-ls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected elixir tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "elixir_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected elixir_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesElm(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("elm-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected elm tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "elm_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected elm_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesEmber(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ember-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected ember tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "ember_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected ember_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesFSharp(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("fsharp-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected fsharp tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "fsharp_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected fsharp_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesFish(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("fish-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected fish tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "fish_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected fish_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesFluentBit(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("fluent-bit-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected fluentbit tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "fluentbit_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected fluentbit_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesFortran(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("fortran-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected fortran tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "fortran_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected fortran_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesFuzion(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("fuzion-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected fuzion tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "fuzion_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected fuzion_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesGLSL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("glsl-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected glsl tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "glsl_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected glsl_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesGLSLMinecraft(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("mcshader-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected mcshader tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "mcshader_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected mcshader_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesGauge(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("gauge-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected gauge tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "gauge_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected gauge_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesGDScript(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("godot4", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected gdscript tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "gdscript_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected gdscript_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesGleam(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("gleam-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected gleam tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "gleam_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected gleam_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesGlimmer(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("glint-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected glimmer tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "glimmer_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected glimmer_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesGluon(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("gluon-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected gluon tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "gluon_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected gluon_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesGN(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("gn-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected gn tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "gn_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected gn_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSourcegraphGo(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("go-langserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected sourcegraphgo tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "sourcegraphgo_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected sourcegraphgo_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesGraphQL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("graphql-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected graphql tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "graphql_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected graphql_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesDot(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("dot-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected dot tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "dot_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected dot_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesGrain(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("grain-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected grain tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "grain_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected grain_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesGroovy(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("groovy-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected groovy tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "groovy_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected groovy_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesHTML(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("vscode-html-languageserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected html tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "html_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected html_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesHaskell(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("haskell-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected haskell tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "haskell_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected haskell_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesHaxe(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("haxe-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected haxe tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "haxe_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected haxe_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesHelm(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("helm-ls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected helm tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "helm_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected helm_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesHLSL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("HlslTools.LanguageServer", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected hlsl tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "hlsl_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected hlsl_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesInk(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ink-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected ink tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "ink_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected ink_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesIsabelle(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("isabelle", []string{"vscode_server"}, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected isabelle tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "isabelle_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected isabelle_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesIdris2(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("idris2-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected idris2 tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "idris2_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected idris2_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesJava(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("jdtls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected java tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "java_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected java_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesJavaScript(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("quick-lint-js", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected javascript tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "javascript_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected javascript_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesFlow(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("flow-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected flow tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "flow_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected flow_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesJSTypescript(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("javascript-typescript-stdio", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected jstypescript tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "jstypescript_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected jstypescript_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesJCL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("jcl-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected jcl tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "jcl_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected jcl_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesJimmerDTO(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("jimmer-dto-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected jimmerdto tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "jimmerdto_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected jimmerdto_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesJSON(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("vscode-json-languageserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected jsonls tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "jsonls_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected jsonls_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesJsonnet(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("jsonnet-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected jsonnet tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "jsonnet_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected jsonnet_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesJulia(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("julia", []string{"--project=/opt/julia-ls", "/opt/julia-ls/src/LanguageServer.jl"}, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected julia tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "julia_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected julia_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesKconfig(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("kconfig-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected kconfig tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "kconfig_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected kconfig_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesKDL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("vscode-kdl", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected kdl tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "kdl_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected kdl_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesKedro(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("kedro-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected kedro tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "kedro_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected kedro_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesKerboScript(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("kos-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected kerboscript tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "kerboscript_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected kerboscript_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesKerML(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("kerml-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected kerml tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "kerml_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected kerml_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesKotlin(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("kotlin-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected kotlin tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "kotlin_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected kotlin_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTypeCobolRobot(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("language-server-robot", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected typecobolrobot tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "typecobolrobot_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected typecobolrobot_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesLanguageTool(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("languagetool-languageserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected languagetool tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "languagetool_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected languagetool_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesLark(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("lark-parser-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected lark tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "lark_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected lark_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesLaTeX(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("texlab", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected latex tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "latex_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected latex_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesLean4(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("Lean4", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected lean4 tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "lean4_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected lean4_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesLox(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("loxcraft", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected lox tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "lox_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected lox_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesLPC(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("lpc-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected lpc tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "lpc_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected lpc_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesLua(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("lua-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected lua tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "lua_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected lua_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesLiquid(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("theme-check-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected liquid tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "liquid_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected liquid_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesLPG(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("LPG-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected lpg tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "lpg_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected lpg_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesMake(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("make-lsp-vscode", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected make tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "make_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected make_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesMarkdown(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("marksman", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected markdown tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "markdown_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected markdown_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesAsmLSP(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("asm-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected asmlsp tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "asmlsp_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected asmlsp_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesMATLAB(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("matlab-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected matlab tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "matlab_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected matlab_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesMDX(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("mdx-analyzer", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected mdx tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "mdx_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected mdx_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesM68k(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("m68k-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected m68k tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "m68k_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected m68k_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesMSBuild(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("msbuild-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected msbuild tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "msbuild_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected msbuild_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesNginx(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("nginx-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected nginx tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "nginx_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected nginx_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesNim(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("nimlsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected nim tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "nim_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected nim_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesNobl9YAML(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("nobl9-vscode", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected nobl9yaml tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "nobl9yaml_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected nobl9yaml_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesOCamlReason(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ocamllsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected ocamlreason tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "ocamlreason_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected ocamlreason_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesOdin(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ols", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected odin tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "odin_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected odin_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesOpenEdgeABL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("abl-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected openedgeabl tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "openedgeabl_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected openedgeabl_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesOpenValidation(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ov-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected openvalidation tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "openvalidation_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected openvalidation_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPapyrus(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("papyrus-lang", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected papyrus tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "papyrus_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected papyrus_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPartiQL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("aws-lsp-partiql", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected partiql tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "partiql_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected partiql_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPerl(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("perl-languageserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected perl tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "perl_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected perl_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPest(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("pest-ide-tools", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected pest tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "pest_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected pest_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPHP(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("intelephense", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected php tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "php_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected php_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPHPUnit(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("phpunit-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected phpunit tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "phpunit_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected phpunit_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPLI(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("pli-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected pli tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "pli_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected pli_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPLSQL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("plsql-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected plsql tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "plsql_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected plsql_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPolymer(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("polymer-editor-service", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected polymer tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "polymer_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected polymer_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPowerPC(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("powerpc-support", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected powerpc tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "powerpc_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected powerpc_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPowerShell(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("powershell-editor-services", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected powershell tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "powershell_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected powershell_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPromQL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("promql-langserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected promql tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "promql_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected promql_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesProtobuf(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("protols", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected protobuf tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "protobuf_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected protobuf_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPureScript(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("purescript-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected purescript tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "purescript_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected purescript_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPuppet(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("puppet-languageserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected puppet tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "puppet_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected puppet_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPython(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("pyright-langserver", []string{"--stdio"}, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected python tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "python_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected python_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPony(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ponyls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected pony tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "pony_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected pony_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesQSharp(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("qsharp-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected qsharp tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "qsharp_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected qsharp_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesQuery(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ts_query_ls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected query tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "query_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected query_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesRLang(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("languageserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected rlang tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "rlang_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected rlang_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesRacket(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("racket-langserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected racket tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "racket_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected racket_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesRain(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("rainlanguageserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected rain tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "rain_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected rain_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesRaku(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("raku-navigator", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected raku tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "raku_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected raku_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesRAML(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("raml-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected raml tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "raml_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected raml_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesRascal(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("rascal-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected rascal tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "rascal_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected rascal_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesReasonML(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("reason-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected reasonml tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "reasonml_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected reasonml_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesRed(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("redlangserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected red tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "red_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected red_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesRego(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("regal-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected rego tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "rego_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected rego_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesREL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("rel-ls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected rel tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "rel_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected rel_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesReScript(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("rescript-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected rescript tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "rescript_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected rescript_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesRexx(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("rexx-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected rexx tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "rexx_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected rexx_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesRobotFramework(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("robotframework-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected robotframework tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "robotframework_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected robotframework_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesRobotsTxt(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("robots-txt-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected robotstxt tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "robotstxt_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected robotstxt_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesRuby(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("solargraph", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected ruby tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "ruby_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected ruby_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesRust(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("rust-analyzer", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected rust tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "rust_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected rust_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesScala(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("metals", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected scala tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "scala_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected scala_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesScheme(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("scheme-langserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected scheme tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "scheme_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected scheme_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesShader(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("shader-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected shader tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "shader_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected shader_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSlint(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("slint-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected slint tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "slint_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected slint_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesPharo(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("pharolanguageserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected pharo tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "pharo_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected pharo_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSmithy(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("smithy-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected smithy tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "smithy_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected smithy_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSnyk(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("snyk-ls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected snyk tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "snyk_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected snyk_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSPARQL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("qlue-ls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected sparql tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "sparql_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected sparql_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSphinx(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("esbonio", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected sphinx tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "sphinx_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected sphinx_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSQL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("sqls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected sql tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "sql_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected sql_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesStandardML(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("millet", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected standardml tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "standardml_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected standardml_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesStimulus(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("stimulus-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected stimulus tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "stimulus_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected stimulus_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesStylable(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("stylable-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected stylable tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "stylable_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected stylable_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSvelte(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("svelteserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected svelte tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "svelte_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected svelte_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSway(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("sway-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected sway tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "sway_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected sway_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSwift(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("sourcekit-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected swift tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "swift_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected swift_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSysML2(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("sysml2-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected sysml2 tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "sysml2_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected sysml2_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSysl(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("sysl-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected sysl tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "sysl_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected sysl_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSystemd(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("systemd-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected systemd tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "systemd_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected systemd_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSystemtap(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("systemtap-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected systemtap tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "systemtap_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected systemtap_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSystemVerilog(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("svls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected systemverilog tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "systemverilog_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected systemverilog_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTSQL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("sqltoolsservice", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected tsql tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "tsql_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected tsql_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTads3(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("tads3-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected tads3 tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "tads3_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected tads3_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTeal(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("teal-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected teal tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "teal_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected teal_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTerraform(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("terraform-ls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected terraform tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "terraform_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected terraform_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesThrift(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("thrift-ls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected thrift tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "thrift_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected thrift_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTibboBasic(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("tibbo-basic-ls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected tibbobasic tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "tibbobasic_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected tibbobasic_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTOML(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("taplo", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected toml tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "toml_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected toml_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTrinoSQL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("trinols", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected trinosql tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "trinosql_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected trinosql_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTTCN3(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("ntt", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected ttcn3 tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "ttcn3_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected ttcn3_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTurtle(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("turtle-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected turtle tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "turtle_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected turtle_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTailwindCSS(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("tailwindcss-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected tailwindcss tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "tailwindcss_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected tailwindcss_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTwig(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("twig-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected twig tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "twig_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected twig_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTypeCobol(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("typecobol-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected typecobol tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "typecobol_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected typecobol_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTypeScriptLS(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("typescript-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected typescriptls tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "typescriptls_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected typescriptls_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTypst(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("tinymist", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected typst tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "typst_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected typst_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesVLang(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("v-analyzer", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected vlang tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "vlang_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected vlang_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesVala(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("vala-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected vala tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "vala_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected vala_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesVDM(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("vdmj-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected vdm tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "vdm_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected vdm_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesVeryl(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("veryl-ls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected veryl tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "veryl_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected veryl_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesVHDL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("vhdl_ls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected vhdl tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "vhdl_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected vhdl_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesViml(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("vim-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected viml tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "viml_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected viml_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesVisualforce(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("visualforce-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected visualforce tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "visualforce_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected visualforce_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesVue(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("vls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected vue tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "vue_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected vue_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesWebAssembly(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("wasm-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected wasm tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "wasm_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected wasm_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesWGSL(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("wgsl_analyzer", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected wgsl tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "wgsl_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected wgsl_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesWikitext(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("wikitext-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected wikitext tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "wikitext_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected wikitext_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesWing(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("wing-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected wing tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "wing_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected wing_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesWolfram(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("lsp-wl", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected wolfram tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "wolfram_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected wolfram_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesWXML(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("wxml-languageserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected wxml tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "wxml_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected wxml_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesXML(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("xml-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected xml tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "xml_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected xml_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesMiniYAML(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("miniyaml-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected miniyaml tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "miniyaml_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected miniyaml_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesYAML(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("yaml-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected yaml tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "yaml_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected yaml_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesYARA(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("yara-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected yara tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "yara_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected yara_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesYANG(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("yang-lsp", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected yang tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "yang_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected yang_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesZig(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("zls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected zig tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "zig_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected zig_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesNix(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("nil", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected nix tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "nix_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected nix_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesEFM(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("efm-langserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected efm tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "efm_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected efm_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesDiagnosticLS(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("diagnostic-languageserver", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected diagnosticls tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "diagnosticls_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected diagnosticls_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTagLS(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("tagls", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected tagls tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "tagls_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected tagls_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesSonarLint(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("sonarlint-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected sonarlint tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "sonarlint_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected sonarlint_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesTestingLS(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("testing-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected testingls tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "testingls_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected testingls_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCopilot(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("copilot-language-server", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected copilot tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "copilot_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected copilot_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesHarper(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("harper", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected harper tools, got none")
	}

	foundExecute := false
	for _, tool := range tools {
		if tool.Name == "harper_execute_extension_command" {
			foundExecute = true
			break
		}
	}
	if !foundExecute {
		t.Fatalf("expected harper_execute_extension_command in tools")
	}
}

func TestBuildToolsMatchesCppTools(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("cpptools-srv", nil, client, ".")
	if len(tools) == 0 {
		t.Fatalf("expected cpptools tools, got none")
	}

	foundSwitch := false
	for _, tool := range tools {
		if tool.Name == "cpptools_switch_header_source" {
			foundSwitch = true
			break
		}
	}
	if !foundSwitch {
		t.Fatalf("expected cpptools_switch_header_source in tools")
	}
}

func TestBuildToolsNoMatch(t *testing.T) {
	client := lsp.NewClient(lsp.Config{Command: "dummy"})
	tools := BuildTools("unknown-lsp", []string{"--stdio"}, client, ".")
	if len(tools) != 0 {
		t.Fatalf("expected no extension tools, got %d", len(tools))
	}
}

func TestDescribeCommonToolScopeMatched(t *testing.T) {
	got := DescribeCommonToolScope("gopls", nil)
	if !strings.Contains(got, "Go (gopls)") {
		t.Fatalf("expected Go profile in scope description, got %q", got)
	}
	if !strings.Contains(got, "capabilities") {
		t.Fatalf("expected capability note in scope description, got %q", got)
	}
}

func TestDescribeCommonToolScopeNoMatch(t *testing.T) {
	got := DescribeCommonToolScope("unknown-lsp", []string{"--stdio"})
	if !strings.Contains(got, "generic LSP") {
		t.Fatalf("expected generic scope description, got %q", got)
	}
}

func TestAllExtensionMeta_ExcludesGenericLanguage(t *testing.T) {
	metas := AllExtensionMeta()
	for _, m := range metas {
		if m.Language == "*" {
			t.Errorf("AllExtensionMeta() included generic language entry: %s", m.Name)
		}
	}
}

func TestAllExtensionMeta_AllHaveRequiredFields(t *testing.T) {
	metas := AllExtensionMeta()
	if len(metas) == 0 {
		t.Fatal("AllExtensionMeta() returned empty list")
	}
	for _, m := range metas {
		if m.DefaultCommand == "" {
			t.Errorf("AllExtensionMeta() entry %q has empty DefaultCommand", m.Name)
		}
		if m.Name == "" {
			t.Error("AllExtensionMeta() entry has empty Name")
		}
		if m.Language == "" {
			t.Errorf("AllExtensionMeta() entry %q has empty Language", m.Name)
		}
	}
}

func TestAllExtensionMeta_DefaultCommandMatchable(t *testing.T) {
	metas := AllExtensionMeta()
	specByName := make(map[string]extensionSpec, len(extensionSpecs))
	for _, spec := range extensionSpecs {
		specByName[spec.name] = spec
	}
	for _, m := range metas {
		spec, ok := specByName[m.Name]
		if !ok {
			t.Errorf("no extensionSpec found for meta %q", m.Name)
			continue
		}
		if !spec.match(m.DefaultCommand, nil) {
			t.Errorf("extensionSpec %q: Matches(%q, nil) = false, want true",
				m.Name, m.DefaultCommand)
		}
	}
}
