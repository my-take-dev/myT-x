package lsppkg

import (
	"strings"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
	"myT-x/internal/mcp/lspmcp/internal/mcp"
	"myT-x/internal/mcp/lspmcp/lsppkg/abap"
	"myT-x/internal/mcp/lspmcp/lsppkg/ada"
	"myT-x/internal/mcp/lspmcp/lsppkg/agda"
	"myT-x/internal/mcp/lspmcp/lsppkg/aml"
	"myT-x/internal/mcp/lspmcp/lsppkg/angular"
	"myT-x/internal/mcp/lspmcp/lsppkg/ansible"
	"myT-x/internal/mcp/lspmcp/lsppkg/antlr"
	"myT-x/internal/mcp/lspmcp/lsppkg/apachedispatcher"
	"myT-x/internal/mcp/lspmcp/lsppkg/apex"
	"myT-x/internal/mcp/lspmcp/lsppkg/apielements"
	"myT-x/internal/mcp/lspmcp/lsppkg/apl"
	"myT-x/internal/mcp/lspmcp/lsppkg/as2"
	"myT-x/internal/mcp/lspmcp/lsppkg/asmlsp"
	"myT-x/internal/mcp/lspmcp/lsppkg/asn1"
	"myT-x/internal/mcp/lspmcp/lsppkg/astro"
	"myT-x/internal/mcp/lspmcp/lsppkg/awk"
	"myT-x/internal/mcp/lspmcp/lsppkg/bake"
	"myT-x/internal/mcp/lspmcp/lsppkg/ballerina"
	"myT-x/internal/mcp/lspmcp/lsppkg/bash"
	"myT-x/internal/mcp/lspmcp/lsppkg/batch"
	"myT-x/internal/mcp/lspmcp/lsppkg/bazel"
	"myT-x/internal/mcp/lspmcp/lsppkg/bicep"
	"myT-x/internal/mcp/lspmcp/lsppkg/bitbake"
	"myT-x/internal/mcp/lspmcp/lsppkg/boriel"
	"myT-x/internal/mcp/lspmcp/lsppkg/bprob"
	"myT-x/internal/mcp/lspmcp/lsppkg/brighterscript"
	"myT-x/internal/mcp/lspmcp/lsppkg/bsl"
	"myT-x/internal/mcp/lspmcp/lsppkg/caddy"
	"myT-x/internal/mcp/lspmcp/lsppkg/camel"
	"myT-x/internal/mcp/lspmcp/lsppkg/ccls"
	"myT-x/internal/mcp/lspmcp/lsppkg/cds"
	"myT-x/internal/mcp/lspmcp/lsppkg/ceylon"
	"myT-x/internal/mcp/lspmcp/lsppkg/chapel"
	"myT-x/internal/mcp/lspmcp/lsppkg/clangd"
	"myT-x/internal/mcp/lspmcp/lsppkg/clarity"
	"myT-x/internal/mcp/lspmcp/lsppkg/clojure"
	"myT-x/internal/mcp/lspmcp/lsppkg/cmake"
	"myT-x/internal/mcp/lspmcp/lsppkg/cobol"
	"myT-x/internal/mcp/lspmcp/lsppkg/codeql"
	"myT-x/internal/mcp/lspmcp/lsppkg/coffeescript"
	"myT-x/internal/mcp/lspmcp/lsppkg/commonlisp"
	"myT-x/internal/mcp/lspmcp/lsppkg/copilot"
	"myT-x/internal/mcp/lspmcp/lsppkg/coq"
	"myT-x/internal/mcp/lspmcp/lsppkg/cpptools"
	"myT-x/internal/mcp/lspmcp/lsppkg/cquery"
	"myT-x/internal/mcp/lspmcp/lsppkg/crystal"
	"myT-x/internal/mcp/lspmcp/lsppkg/csharp"
	"myT-x/internal/mcp/lspmcp/lsppkg/cssls"
	"myT-x/internal/mcp/lspmcp/lsppkg/cucumber"
	"myT-x/internal/mcp/lspmcp/lsppkg/cwl"
	"myT-x/internal/mcp/lspmcp/lsppkg/cython"
	"myT-x/internal/mcp/lspmcp/lsppkg/dart"
	"myT-x/internal/mcp/lspmcp/lsppkg/datapack"
	"myT-x/internal/mcp/lspmcp/lsppkg/debian"
	"myT-x/internal/mcp/lspmcp/lsppkg/delphi"
	"myT-x/internal/mcp/lspmcp/lsppkg/denizenscript"
	"myT-x/internal/mcp/lspmcp/lsppkg/deno"
	"myT-x/internal/mcp/lspmcp/lsppkg/devicetree"
	"myT-x/internal/mcp/lspmcp/lsppkg/diagnosticls"
	"myT-x/internal/mcp/lspmcp/lsppkg/dlang"
	"myT-x/internal/mcp/lspmcp/lsppkg/dockerfile"
	"myT-x/internal/mcp/lspmcp/lsppkg/dot"
	"myT-x/internal/mcp/lspmcp/lsppkg/dreammaker"
	"myT-x/internal/mcp/lspmcp/lsppkg/efm"
	"myT-x/internal/mcp/lspmcp/lsppkg/egglog"
	"myT-x/internal/mcp/lspmcp/lsppkg/elixir"
	"myT-x/internal/mcp/lspmcp/lsppkg/elm"
	"myT-x/internal/mcp/lspmcp/lsppkg/emacslisp"
	"myT-x/internal/mcp/lspmcp/lsppkg/ember"
	"myT-x/internal/mcp/lspmcp/lsppkg/erg"
	"myT-x/internal/mcp/lspmcp/lsppkg/erlang"
	"myT-x/internal/mcp/lspmcp/lsppkg/fish"
	"myT-x/internal/mcp/lspmcp/lsppkg/flow"
	"myT-x/internal/mcp/lspmcp/lsppkg/fluentbit"
	"myT-x/internal/mcp/lspmcp/lsppkg/fortran"
	"myT-x/internal/mcp/lspmcp/lsppkg/fsharp"
	"myT-x/internal/mcp/lspmcp/lsppkg/fuzion"
	"myT-x/internal/mcp/lspmcp/lsppkg/gauge"
	"myT-x/internal/mcp/lspmcp/lsppkg/gdscript"
	"myT-x/internal/mcp/lspmcp/lsppkg/gleam"
	"myT-x/internal/mcp/lspmcp/lsppkg/glimmer"
	"myT-x/internal/mcp/lspmcp/lsppkg/glsl"
	"myT-x/internal/mcp/lspmcp/lsppkg/gluon"
	"myT-x/internal/mcp/lspmcp/lsppkg/gn"
	"myT-x/internal/mcp/lspmcp/lsppkg/gopls"
	"myT-x/internal/mcp/lspmcp/lsppkg/grain"
	"myT-x/internal/mcp/lspmcp/lsppkg/graphql"
	"myT-x/internal/mcp/lspmcp/lsppkg/groovy"
	"myT-x/internal/mcp/lspmcp/lsppkg/harper"
	"myT-x/internal/mcp/lspmcp/lsppkg/haskell"
	"myT-x/internal/mcp/lspmcp/lsppkg/haxe"
	"myT-x/internal/mcp/lspmcp/lsppkg/helm"
	"myT-x/internal/mcp/lspmcp/lsppkg/hlasm"
	"myT-x/internal/mcp/lspmcp/lsppkg/hlsl"
	"myT-x/internal/mcp/lspmcp/lsppkg/html"
	"myT-x/internal/mcp/lspmcp/lsppkg/ibmi"
	"myT-x/internal/mcp/lspmcp/lsppkg/idris2"
	"myT-x/internal/mcp/lspmcp/lsppkg/ink"
	"myT-x/internal/mcp/lspmcp/lsppkg/isabelle"
	"myT-x/internal/mcp/lspmcp/lsppkg/java"
	"myT-x/internal/mcp/lspmcp/lsppkg/javascript"
	"myT-x/internal/mcp/lspmcp/lsppkg/jcl"
	"myT-x/internal/mcp/lspmcp/lsppkg/jimmerdto"
	"myT-x/internal/mcp/lspmcp/lsppkg/jsonls"
	"myT-x/internal/mcp/lspmcp/lsppkg/jsonnet"
	"myT-x/internal/mcp/lspmcp/lsppkg/jstypescript"
	"myT-x/internal/mcp/lspmcp/lsppkg/julia"
	"myT-x/internal/mcp/lspmcp/lsppkg/kconfig"
	"myT-x/internal/mcp/lspmcp/lsppkg/kdl"
	"myT-x/internal/mcp/lspmcp/lsppkg/kedro"
	"myT-x/internal/mcp/lspmcp/lsppkg/kerboscript"
	"myT-x/internal/mcp/lspmcp/lsppkg/kerml"
	"myT-x/internal/mcp/lspmcp/lsppkg/kotlin"
	"myT-x/internal/mcp/lspmcp/lsppkg/languagetool"
	"myT-x/internal/mcp/lspmcp/lsppkg/lark"
	"myT-x/internal/mcp/lspmcp/lsppkg/latex"
	"myT-x/internal/mcp/lspmcp/lsppkg/lean4"
	"myT-x/internal/mcp/lspmcp/lsppkg/liquid"
	"myT-x/internal/mcp/lspmcp/lsppkg/lox"
	"myT-x/internal/mcp/lspmcp/lsppkg/lpc"
	"myT-x/internal/mcp/lspmcp/lsppkg/lpg"
	"myT-x/internal/mcp/lspmcp/lsppkg/lua"
	"myT-x/internal/mcp/lspmcp/lsppkg/m68k"
	makelsp "myT-x/internal/mcp/lspmcp/lsppkg/make"
	"myT-x/internal/mcp/lspmcp/lsppkg/markdown"
	"myT-x/internal/mcp/lspmcp/lsppkg/matlab"
	"myT-x/internal/mcp/lspmcp/lsppkg/mcshader"
	"myT-x/internal/mcp/lspmcp/lsppkg/mdx"
	"myT-x/internal/mcp/lspmcp/lsppkg/miniyaml"
	"myT-x/internal/mcp/lspmcp/lsppkg/msbuild"
	"myT-x/internal/mcp/lspmcp/lsppkg/nginx"
	"myT-x/internal/mcp/lspmcp/lsppkg/nim"
	"myT-x/internal/mcp/lspmcp/lsppkg/nix"
	"myT-x/internal/mcp/lspmcp/lsppkg/nobl9yaml"
	"myT-x/internal/mcp/lspmcp/lsppkg/ocamlreason"
	"myT-x/internal/mcp/lspmcp/lsppkg/odin"
	"myT-x/internal/mcp/lspmcp/lsppkg/openedgeabl"
	"myT-x/internal/mcp/lspmcp/lsppkg/openvalidation"
	"myT-x/internal/mcp/lspmcp/lsppkg/papyrus"
	"myT-x/internal/mcp/lspmcp/lsppkg/partiql"
	"myT-x/internal/mcp/lspmcp/lsppkg/perl"
	"myT-x/internal/mcp/lspmcp/lsppkg/pest"
	"myT-x/internal/mcp/lspmcp/lsppkg/pharo"
	"myT-x/internal/mcp/lspmcp/lsppkg/php"
	"myT-x/internal/mcp/lspmcp/lsppkg/phpunit"
	"myT-x/internal/mcp/lspmcp/lsppkg/pli"
	"myT-x/internal/mcp/lspmcp/lsppkg/plsql"
	"myT-x/internal/mcp/lspmcp/lsppkg/polymer"
	"myT-x/internal/mcp/lspmcp/lsppkg/pony"
	"myT-x/internal/mcp/lspmcp/lsppkg/powerpc"
	"myT-x/internal/mcp/lspmcp/lsppkg/powershell"
	"myT-x/internal/mcp/lspmcp/lsppkg/promql"
	"myT-x/internal/mcp/lspmcp/lsppkg/protobuf"
	"myT-x/internal/mcp/lspmcp/lsppkg/puppet"
	"myT-x/internal/mcp/lspmcp/lsppkg/purescript"
	"myT-x/internal/mcp/lspmcp/lsppkg/python"
	"myT-x/internal/mcp/lspmcp/lsppkg/qmlls"
	"myT-x/internal/mcp/lspmcp/lsppkg/qsharp"
	"myT-x/internal/mcp/lspmcp/lsppkg/query"
	"myT-x/internal/mcp/lspmcp/lsppkg/racket"
	"myT-x/internal/mcp/lspmcp/lsppkg/rain"
	"myT-x/internal/mcp/lspmcp/lsppkg/raku"
	"myT-x/internal/mcp/lspmcp/lsppkg/raml"
	"myT-x/internal/mcp/lspmcp/lsppkg/rascal"
	"myT-x/internal/mcp/lspmcp/lsppkg/reasonml"
	"myT-x/internal/mcp/lspmcp/lsppkg/red"
	"myT-x/internal/mcp/lspmcp/lsppkg/rego"
	"myT-x/internal/mcp/lspmcp/lsppkg/rel"
	"myT-x/internal/mcp/lspmcp/lsppkg/rescript"
	"myT-x/internal/mcp/lspmcp/lsppkg/rexx"
	"myT-x/internal/mcp/lspmcp/lsppkg/rlang"
	"myT-x/internal/mcp/lspmcp/lsppkg/robotframework"
	"myT-x/internal/mcp/lspmcp/lsppkg/robotstxt"
	"myT-x/internal/mcp/lspmcp/lsppkg/ruby"
	"myT-x/internal/mcp/lspmcp/lsppkg/rust"
	"myT-x/internal/mcp/lspmcp/lsppkg/scala"
	"myT-x/internal/mcp/lspmcp/lsppkg/scheme"
	"myT-x/internal/mcp/lspmcp/lsppkg/shader"
	"myT-x/internal/mcp/lspmcp/lsppkg/slint"
	"myT-x/internal/mcp/lspmcp/lsppkg/smithy"
	"myT-x/internal/mcp/lspmcp/lsppkg/snyk"
	"myT-x/internal/mcp/lspmcp/lsppkg/sonarlint"
	"myT-x/internal/mcp/lspmcp/lsppkg/sourcegraphgo"
	"myT-x/internal/mcp/lspmcp/lsppkg/sparql"
	"myT-x/internal/mcp/lspmcp/lsppkg/sphinx"
	"myT-x/internal/mcp/lspmcp/lsppkg/sql"
	"myT-x/internal/mcp/lspmcp/lsppkg/standardml"
	"myT-x/internal/mcp/lspmcp/lsppkg/stimulus"
	"myT-x/internal/mcp/lspmcp/lsppkg/stylable"
	"myT-x/internal/mcp/lspmcp/lsppkg/svelte"
	"myT-x/internal/mcp/lspmcp/lsppkg/sway"
	"myT-x/internal/mcp/lspmcp/lsppkg/swift"
	"myT-x/internal/mcp/lspmcp/lsppkg/sysl"
	"myT-x/internal/mcp/lspmcp/lsppkg/sysml2"
	"myT-x/internal/mcp/lspmcp/lsppkg/systemd"
	"myT-x/internal/mcp/lspmcp/lsppkg/systemtap"
	"myT-x/internal/mcp/lspmcp/lsppkg/systemverilog"
	"myT-x/internal/mcp/lspmcp/lsppkg/tads3"
	"myT-x/internal/mcp/lspmcp/lsppkg/tagls"
	"myT-x/internal/mcp/lspmcp/lsppkg/tailwindcss"
	"myT-x/internal/mcp/lspmcp/lsppkg/teal"
	"myT-x/internal/mcp/lspmcp/lsppkg/terraform"
	"myT-x/internal/mcp/lspmcp/lsppkg/testingls"
	"myT-x/internal/mcp/lspmcp/lsppkg/thrift"
	"myT-x/internal/mcp/lspmcp/lsppkg/tibbobasic"
	"myT-x/internal/mcp/lspmcp/lsppkg/toml"
	"myT-x/internal/mcp/lspmcp/lsppkg/trinosql"
	"myT-x/internal/mcp/lspmcp/lsppkg/tsql"
	"myT-x/internal/mcp/lspmcp/lsppkg/ttcn3"
	"myT-x/internal/mcp/lspmcp/lsppkg/turtle"
	"myT-x/internal/mcp/lspmcp/lsppkg/twig"
	"myT-x/internal/mcp/lspmcp/lsppkg/typecobol"
	"myT-x/internal/mcp/lspmcp/lsppkg/typecobolrobot"
	"myT-x/internal/mcp/lspmcp/lsppkg/typescriptls"
	"myT-x/internal/mcp/lspmcp/lsppkg/typst"
	"myT-x/internal/mcp/lspmcp/lsppkg/vala"
	"myT-x/internal/mcp/lspmcp/lsppkg/vdm"
	"myT-x/internal/mcp/lspmcp/lsppkg/veryl"
	"myT-x/internal/mcp/lspmcp/lsppkg/vhdl"
	"myT-x/internal/mcp/lspmcp/lsppkg/viml"
	"myT-x/internal/mcp/lspmcp/lsppkg/visualforce"
	"myT-x/internal/mcp/lspmcp/lsppkg/vlang"
	"myT-x/internal/mcp/lspmcp/lsppkg/vue"
	"myT-x/internal/mcp/lspmcp/lsppkg/wasm"
	"myT-x/internal/mcp/lspmcp/lsppkg/wgsl"
	"myT-x/internal/mcp/lspmcp/lsppkg/wikitext"
	"myT-x/internal/mcp/lspmcp/lsppkg/wing"
	"myT-x/internal/mcp/lspmcp/lsppkg/wolfram"
	"myT-x/internal/mcp/lspmcp/lsppkg/wxml"
	"myT-x/internal/mcp/lspmcp/lsppkg/xml"
	"myT-x/internal/mcp/lspmcp/lsppkg/yaml"
	"myT-x/internal/mcp/lspmcp/lsppkg/yang"
	"myT-x/internal/mcp/lspmcp/lsppkg/yara"
	"myT-x/internal/mcp/lspmcp/lsppkg/zig"
)

type extensionSpec struct {
	name           string
	language       string
	defaultCommand string // canonical LSP binary name for MCP Definition registration
	match          func(command string, args []string) bool
	build          func(client *lsp.Client, rootDir string) []mcp.Tool
}

var extensionSpecs = []extensionSpec{
	{
		name:           "abap",
		language:       "ABAP",
		defaultCommand: "abaplint",
		match:          abap.Matches,
		build:          abap.BuildTools,
	},
	{
		name:           "as2",
		language:       "ActionScript 2.0",
		defaultCommand: "as2-language-server",
		match:          as2.Matches,
		build:          as2.BuildTools,
	},
	{
		name:           "asn1",
		language:       "ASN.1",
		defaultCommand: "titan-language-server",
		match:          asn1.Matches,
		build:          asn1.BuildTools,
	},
	{
		name:           "ada",
		language:       "Ada/SPARK",
		defaultCommand: "ada_language_server",
		match:          ada.Matches,
		build:          ada.BuildTools,
	},
	{
		name:           "agda",
		language:       "Agda",
		defaultCommand: "agda-language-server",
		match:          agda.Matches,
		build:          agda.BuildTools,
	},
	{
		name:           "aml",
		language:       "AML",
		defaultCommand: "aml-language-server",
		match:          aml.Matches,
		build:          aml.BuildTools,
	},
	{
		name:           "ansible",
		language:       "Ansible",
		defaultCommand: "ansible-language-server",
		match:          ansible.Matches,
		build:          ansible.BuildTools,
	},
	{
		name:           "angular",
		language:       "Angular",
		defaultCommand: "ngserver",
		match:          angular.Matches,
		build:          angular.BuildTools,
	},
	{
		name:           "antlr",
		language:       "Antlr",
		defaultCommand: "antlr-language-server",
		match:          antlr.Matches,
		build:          antlr.BuildTools,
	},
	{
		name:           "apielements",
		language:       "API Elements",
		defaultCommand: "apielements-language-server",
		match:          apielements.Matches,
		build:          apielements.BuildTools,
	},
	{
		name:           "apl",
		language:       "APL",
		defaultCommand: "apl-language-server",
		match:          apl.Matches,
		build:          apl.BuildTools,
	},
	{
		name:           "asmlsp",
		language:       "NASM/GO/GAS Assembly",
		defaultCommand: "asm-lsp",
		match:          asmlsp.Matches,
		build:          asmlsp.BuildTools,
	},
	{
		name:           "camel",
		language:       "Apache Camel",
		defaultCommand: "camel-language-server",
		match:          camel.Matches,
		build:          camel.BuildTools,
	},
	{
		name:           "apachedispatcher",
		language:       "Apache Dispatcher Config",
		defaultCommand: "apache-dispatcher-config-language-server",
		match:          apachedispatcher.Matches,
		build:          apachedispatcher.BuildTools,
	},
	{
		name:           "apex",
		language:       "Apex",
		defaultCommand: "apex-jorje-lsp",
		match:          apex.Matches,
		build:          apex.BuildTools,
	},
	{
		name:           "astro",
		language:       "Astro",
		defaultCommand: "astro-ls",
		match:          astro.Matches,
		build:          astro.BuildTools,
	},
	{
		name:           "awk",
		language:       "AWK",
		defaultCommand: "awk-language-server",
		match:          awk.Matches,
		build:          awk.BuildTools,
	},
	{
		name:           "bake",
		language:       "Bake",
		defaultCommand: "docker-language-server",
		match:          bake.Matches,
		build:          bake.BuildTools,
	},
	{
		name:           "ballerina",
		language:       "Ballerina",
		defaultCommand: "ballerina-language-server",
		match:          ballerina.Matches,
		build:          ballerina.BuildTools,
	},
	{
		name:           "bash",
		language:       "Bash",
		defaultCommand: "bash-language-server",
		match:          bash.Matches,
		build:          bash.BuildTools,
	},
	{
		name:           "batch",
		language:       "Batch",
		defaultCommand: "rech-editor-batch",
		match:          batch.Matches,
		build:          batch.BuildTools,
	},
	{
		name:           "bazel",
		language:       "Bazel",
		defaultCommand: "bazel-lsp",
		match:          bazel.Matches,
		build:          bazel.BuildTools,
	},
	{
		name:           "bicep",
		language:       "Bicep",
		defaultCommand: "bicep-language-server",
		match:          bicep.Matches,
		build:          bicep.BuildTools,
	},
	{
		name:           "bitbake",
		language:       "BitBake",
		defaultCommand: "bitbake-language-server",
		match:          bitbake.Matches,
		build:          bitbake.BuildTools,
	},
	{
		name:           "bsl",
		language:       "1C Enterprise (BSL)",
		defaultCommand: "bsl-language-server",
		match:          bsl.Matches,
		build:          bsl.BuildTools,
	},
	{
		name:           "boriel",
		language:       "Boriel Basic",
		defaultCommand: "boriel-basic-lsp",
		match:          boriel.Matches,
		build:          boriel.BuildTools,
	},
	{
		name:           "bprob",
		language:       "B/ProB",
		defaultCommand: "b-language-server",
		match:          bprob.Matches,
		build:          bprob.BuildTools,
	},
	{
		name:           "brighterscript",
		language:       "BrightScript/BrighterScript",
		defaultCommand: "brighterscript-language-server",
		match:          brighterscript.Matches,
		build:          brighterscript.BuildTools,
	},
	{
		name:           "caddy",
		language:       "caddy",
		defaultCommand: "caddyfile-language-server",
		match:          caddy.Matches,
		build:          caddy.BuildTools,
	},
	{
		name:           "cds",
		language:       "CDS",
		defaultCommand: "cds-lsp",
		match:          cds.Matches,
		build:          cds.BuildTools,
	},
	{
		name:           "cssls",
		language:       "CSS/LESS/SASS",
		defaultCommand: "vscode-css-language-server",
		match:          cssls.Matches,
		build:          cssls.BuildTools,
	},
	{
		name:           "ceylon",
		language:       "Ceylon",
		defaultCommand: "ceylon-language-server",
		match:          ceylon.Matches,
		build:          ceylon.BuildTools,
	},
	{
		name:           "clarity",
		language:       "Clarity",
		defaultCommand: "clarity-lsp",
		match:          clarity.Matches,
		build:          clarity.BuildTools,
	},
	{
		name:           "clojure",
		language:       "Clojure",
		defaultCommand: "clojure-lsp",
		match:          clojure.Matches,
		build:          clojure.BuildTools,
	},
	{
		name:           "cmake",
		language:       "CMake",
		defaultCommand: "cmake-language-server",
		match:          cmake.Matches,
		build:          cmake.BuildTools,
	},
	{
		name:           "commonlisp",
		language:       "Common Lisp",
		defaultCommand: "cl-lsp",
		match:          commonlisp.Matches,
		build:          commonlisp.BuildTools,
	},
	{
		name:           "chapel",
		language:       "Chapel",
		defaultCommand: "chapel-language-server",
		match:          chapel.Matches,
		build:          chapel.BuildTools,
	},
	{
		name:           "coq",
		language:       "Coq",
		defaultCommand: "coq-lsp",
		match:          coq.Matches,
		build:          coq.BuildTools,
	},
	{
		name:           "cobol",
		language:       "COBOL",
		defaultCommand: "cobol-language-server",
		match:          cobol.Matches,
		build:          cobol.BuildTools,
	},
	{
		name:           "codeql",
		language:       "CodeQL",
		defaultCommand: "codeql-language-server",
		match:          codeql.Matches,
		build:          codeql.BuildTools,
	},
	{
		name:           "coffeescript",
		language:       "CoffeeScript",
		defaultCommand: "coffeesense",
		match:          coffeescript.Matches,
		build:          coffeescript.BuildTools,
	},
	{
		name:           "clangd",
		language:       "C/C++",
		defaultCommand: "clangd",
		match:          clangd.Matches,
		build:          clangd.BuildTools,
	},
	{
		name:           "csharp",
		language:       "C#",
		defaultCommand: "omnisharp",
		match:          csharp.Matches,
		build:          csharp.BuildTools,
	},
	{
		name:           "ccls",
		language:       "C/C++",
		defaultCommand: "ccls",
		match:          ccls.Matches,
		build:          ccls.BuildTools,
	},
	{
		name:           "cquery",
		language:       "C/C++",
		defaultCommand: "cquery",
		match:          cquery.Matches,
		build:          cquery.BuildTools,
	},
	{
		name:           "crystal",
		language:       "Crystal",
		defaultCommand: "crystalline",
		match:          crystal.Matches,
		build:          crystal.BuildTools,
	},
	{
		name:           "cpptools",
		language:       "C/C++",
		defaultCommand: "cpptools-srv",
		match:          cpptools.Matches,
		build:          cpptools.BuildTools,
	},
	{
		name:           "cwl",
		language:       "CWL",
		defaultCommand: "cwl-language-server",
		match:          cwl.Matches,
		build:          cwl.BuildTools,
	},
	{
		name:           "cucumber",
		language:       "Cucumber/Gherkin",
		defaultCommand: "cucumber-language-server",
		match:          cucumber.Matches,
		build:          cucumber.BuildTools,
	},
	{
		name:           "cython",
		language:       "Cython",
		defaultCommand: "cyright-langserver",
		match:          cython.Matches,
		build:          cython.BuildTools,
	},
	{
		name:           "dlang",
		language:       "D",
		defaultCommand: "serve-d",
		match:          dlang.Matches,
		build:          dlang.BuildTools,
	},
	{
		name:           "dot",
		language:       "Graphviz/DOT",
		defaultCommand: "dot-language-server",
		match:          dot.Matches,
		build:          dot.BuildTools,
	},
	{
		name:           "dart",
		language:       "Dart",
		defaultCommand: "dart-language-server",
		match:          dart.Matches,
		build:          dart.BuildTools,
	},
	{
		name:           "datapack",
		language:       "Data Pack",
		defaultCommand: "datapack-language-server",
		match:          datapack.Matches,
		build:          datapack.BuildTools,
	},
	{
		name:           "debian",
		language:       "Debian Packaging files",
		defaultCommand: "debputy-lsp",
		match:          debian.Matches,
		build:          debian.BuildTools,
	},
	{
		name:           "delphi",
		language:       "Delphi",
		defaultCommand: "delphilsp",
		match:          delphi.Matches,
		build:          delphi.BuildTools,
	},
	{
		name:           "denizenscript",
		language:       "DenizenScript",
		defaultCommand: "denizen-language-server",
		match:          denizenscript.Matches,
		build:          denizenscript.BuildTools,
	},
	{
		name:           "devicetree",
		language:       "devicetree",
		defaultCommand: "dts-lsp",
		match:          devicetree.Matches,
		build:          devicetree.BuildTools,
	},
	{
		name:           "deno",
		language:       "Deno (TypeScript/JavaScript)",
		defaultCommand: "denols",
		match:          deno.Matches,
		build:          deno.BuildTools,
	},
	{
		name:           "dockerfile",
		language:       "Dockerfiles",
		defaultCommand: "docker-langserver",
		match:          dockerfile.Matches,
		build:          dockerfile.BuildTools,
	},
	{
		name:           "dreammaker",
		language:       "DreamMaker",
		defaultCommand: "dm-langserver",
		match:          dreammaker.Matches,
		build:          dreammaker.BuildTools,
	},
	{
		name:           "egglog",
		language:       "Egglog",
		defaultCommand: "egglog-language-server",
		match:          egglog.Matches,
		build:          egglog.BuildTools,
	},
	{
		name:           "emacslisp",
		language:       "Emacs Lisp",
		defaultCommand: "ellsp",
		match:          emacslisp.Matches,
		build:          emacslisp.BuildTools,
	},
	{
		name:           "erlang",
		language:       "Erlang",
		defaultCommand: "erlang_ls",
		match:          erlang.Matches,
		build:          erlang.BuildTools,
	},
	{
		name:           "flow",
		language:       "JavaScript Flow",
		defaultCommand: "flow-language-server",
		match:          flow.Matches,
		build:          flow.BuildTools,
	},
	{
		name:           "erg",
		language:       "Erg",
		defaultCommand: "els",
		match:          erg.Matches,
		build:          erg.BuildTools,
	},
	{
		name:           "elixir",
		language:       "Elixir",
		defaultCommand: "elixir-ls",
		match:          elixir.Matches,
		build:          elixir.BuildTools,
	},
	{
		name:           "elm",
		language:       "Elm",
		defaultCommand: "elm-language-server",
		match:          elm.Matches,
		build:          elm.BuildTools,
	},
	{
		name:           "ember",
		language:       "Ember",
		defaultCommand: "ember-language-server",
		match:          ember.Matches,
		build:          ember.BuildTools,
	},
	{
		name:           "fsharp",
		language:       "F#",
		defaultCommand: "fsharp-language-server",
		match:          fsharp.Matches,
		build:          fsharp.BuildTools,
	},
	{
		name:           "fish",
		language:       "fish",
		defaultCommand: "fish-lsp",
		match:          fish.Matches,
		build:          fish.BuildTools,
	},
	{
		name:           "fluentbit",
		language:       "fluent-bit",
		defaultCommand: "fluent-bit-lsp",
		match:          fluentbit.Matches,
		build:          fluentbit.BuildTools,
	},
	{
		name:           "fortran",
		language:       "Fortran",
		defaultCommand: "fortran-language-server",
		match:          fortran.Matches,
		build:          fortran.BuildTools,
	},
	{
		name:           "fuzion",
		language:       "Fuzion",
		defaultCommand: "fuzion-lsp",
		match:          fuzion.Matches,
		build:          fuzion.BuildTools,
	},
	{
		name:           "glsl",
		language:       "GLSL",
		defaultCommand: "glsl-language-server",
		match:          glsl.Matches,
		build:          glsl.BuildTools,
	},
	{
		name:           "mcshader",
		language:       "GLSL for Minecraft",
		defaultCommand: "mcshader-lsp",
		match:          mcshader.Matches,
		build:          mcshader.BuildTools,
	},
	{
		name:           "gauge",
		language:       "Gauge",
		defaultCommand: "gauge-lsp",
		match:          gauge.Matches,
		build:          gauge.BuildTools,
	},
	{
		name:           "gdscript",
		language:       "GDScript",
		defaultCommand: "godot4",
		match:          gdscript.Matches,
		build:          gdscript.BuildTools,
	},
	{
		name:           "gleam",
		language:       "Gleam",
		defaultCommand: "gleam-lsp",
		match:          gleam.Matches,
		build:          gleam.BuildTools,
	},
	{
		name:           "glimmer",
		language:       "Glimmer templates",
		defaultCommand: "glint-language-server",
		match:          glimmer.Matches,
		build:          glimmer.BuildTools,
	},
	{
		name:           "gluon",
		language:       "Gluon",
		defaultCommand: "gluon-language-server",
		match:          gluon.Matches,
		build:          gluon.BuildTools,
	},
	{
		name:           "gn",
		language:       "GN",
		defaultCommand: "gn-language-server",
		match:          gn.Matches,
		build:          gn.BuildTools,
	},
	{
		name:           "grain",
		language:       "Grain",
		defaultCommand: "grain-language-server",
		match:          grain.Matches,
		build:          grain.BuildTools,
	},
	{
		name:           "graphql",
		language:       "GraphQL",
		defaultCommand: "graphql-lsp",
		match:          graphql.Matches,
		build:          graphql.BuildTools,
	},
	{
		name:           "groovy",
		language:       "Groovy",
		defaultCommand: "groovy-language-server",
		match:          groovy.Matches,
		build:          groovy.BuildTools,
	},
	{
		name:           "html",
		language:       "HTML",
		defaultCommand: "vscode-html-languageserver",
		match:          html.Matches,
		build:          html.BuildTools,
	},
	{
		name:           "haskell",
		language:       "Haskell",
		defaultCommand: "haskell-language-server",
		match:          haskell.Matches,
		build:          haskell.BuildTools,
	},
	{
		name:           "haxe",
		language:       "Haxe",
		defaultCommand: "haxe-language-server",
		match:          haxe.Matches,
		build:          haxe.BuildTools,
	},
	{
		name:           "helm",
		language:       "Helm (Kubernetes)",
		defaultCommand: "helm-ls",
		match:          helm.Matches,
		build:          helm.BuildTools,
	},
	{
		name:           "hlsl",
		language:       "HLSL",
		defaultCommand: "HlslTools.LanguageServer",
		match:          hlsl.Matches,
		build:          hlsl.BuildTools,
	},
	{
		name:           "ink",
		language:       "ink!",
		defaultCommand: "ink-language-server",
		match:          ink.Matches,
		build:          ink.BuildTools,
	},
	{
		name:           "isabelle",
		language:       "Isabelle",
		defaultCommand: "isabelle-vscode-server",
		match:          isabelle.Matches,
		build:          isabelle.BuildTools,
	},
	{
		name:           "idris2",
		language:       "Idris2",
		defaultCommand: "idris2-lsp",
		match:          idris2.Matches,
		build:          idris2.BuildTools,
	},
	{
		name:           "java",
		language:       "Java",
		defaultCommand: "jdtls",
		match:          java.Matches,
		build:          java.BuildTools,
	},
	{
		name:           "javascript",
		language:       "JavaScript",
		defaultCommand: "quick-lint-js",
		match:          javascript.Matches,
		build:          javascript.BuildTools,
	},
	{
		name:           "jstypescript",
		language:       "JavaScript-Typescript",
		defaultCommand: "javascript-typescript-stdio",
		match:          jstypescript.Matches,
		build:          jstypescript.BuildTools,
	},
	{
		name:           "jcl",
		language:       "JCL",
		defaultCommand: "jcl-language-server",
		match:          jcl.Matches,
		build:          jcl.BuildTools,
	},
	{
		name:           "jimmerdto",
		language:       "Jimmer DTO",
		defaultCommand: "jimmer-dto-lsp",
		match:          jimmerdto.Matches,
		build:          jimmerdto.BuildTools,
	},
	{
		name:           "jsonls",
		language:       "JSON",
		defaultCommand: "vscode-json-languageserver",
		match:          jsonls.Matches,
		build:          jsonls.BuildTools,
	},
	{
		name:           "jsonnet",
		language:       "Jsonnet",
		defaultCommand: "jsonnet-language-server",
		match:          jsonnet.Matches,
		build:          jsonnet.BuildTools,
	},
	{
		name:           "julia",
		language:       "Julia",
		defaultCommand: "julia-language-server",
		match:          julia.Matches,
		build:          julia.BuildTools,
	},
	{
		name:           "kconfig",
		language:       "Kconfig",
		defaultCommand: "kconfig-language-server",
		match:          kconfig.Matches,
		build:          kconfig.BuildTools,
	},
	{
		name:           "kdl",
		language:       "KDL",
		defaultCommand: "vscode-kdl",
		match:          kdl.Matches,
		build:          kdl.BuildTools,
	},
	{
		name:           "kedro",
		language:       "Kedro",
		defaultCommand: "kedro-language-server",
		match:          kedro.Matches,
		build:          kedro.BuildTools,
	},
	{
		name:           "kerboscript",
		language:       "Kerboscript (kOS)",
		defaultCommand: "kos-language-server",
		match:          kerboscript.Matches,
		build:          kerboscript.BuildTools,
	},
	{
		name:           "kerml",
		language:       "KerML",
		defaultCommand: "kerml-language-server",
		match:          kerml.Matches,
		build:          kerml.BuildTools,
	},
	{
		name:           "kotlin",
		language:       "Kotlin",
		defaultCommand: "kotlin-language-server",
		match:          kotlin.Matches,
		build:          kotlin.BuildTools,
	},
	{
		name:           "typecobolrobot",
		language:       "Language Server Robot",
		defaultCommand: "language-server-robot",
		match:          typecobolrobot.Matches,
		build:          typecobolrobot.BuildTools,
	},
	{
		name:           "languagetool",
		language:       "LanguageTool",
		defaultCommand: "languagetool-languageserver",
		match:          languagetool.Matches,
		build:          languagetool.BuildTools,
	},
	{
		name:           "lark",
		language:       "Lark",
		defaultCommand: "lark-parser-language-server",
		match:          lark.Matches,
		build:          lark.BuildTools,
	},
	{
		name:           "latex",
		language:       "LaTeX",
		defaultCommand: "texlab",
		match:          latex.Matches,
		build:          latex.BuildTools,
	},
	{
		name:           "lean4",
		language:       "Lean4",
		defaultCommand: "Lean4",
		match:          lean4.Matches,
		build:          lean4.BuildTools,
	},
	{
		name:           "lox",
		language:       "Lox",
		defaultCommand: "loxcraft",
		match:          lox.Matches,
		build:          lox.BuildTools,
	},
	{
		name:           "lpc",
		language:       "LPC",
		defaultCommand: "lpc-language-server",
		match:          lpc.Matches,
		build:          lpc.BuildTools,
	},
	{
		name:           "lua",
		language:       "Lua",
		defaultCommand: "lua-language-server",
		match:          lua.Matches,
		build:          lua.BuildTools,
	},
	{
		name:           "liquid",
		language:       "Liquid",
		defaultCommand: "theme-check-language-server",
		match:          liquid.Matches,
		build:          liquid.BuildTools,
	},
	{
		name:           "lpg",
		language:       "IBM LALR Parser Generator",
		defaultCommand: "LPG-language-server",
		match:          lpg.Matches,
		build:          lpg.BuildTools,
	},
	{
		name:           "make",
		language:       "Make",
		defaultCommand: "make-lsp-vscode",
		match:          makelsp.Matches,
		build:          makelsp.BuildTools,
	},
	{
		name:           "markdown",
		language:       "Markdown",
		defaultCommand: "marksman",
		match:          markdown.Matches,
		build:          markdown.BuildTools,
	},
	{
		name:           "matlab",
		language:       "MATLAB",
		defaultCommand: "matlab-language-server",
		match:          matlab.Matches,
		build:          matlab.BuildTools,
	},
	{
		name:           "mdx",
		language:       "MDX",
		defaultCommand: "mdx-analyzer",
		match:          mdx.Matches,
		build:          mdx.BuildTools,
	},
	{
		name:           "m68k",
		language:       "Motorola 68000 Assembly",
		defaultCommand: "m68k-lsp",
		match:          m68k.Matches,
		build:          m68k.BuildTools,
	},
	{
		name:           "msbuild",
		language:       "MSBuild",
		defaultCommand: "msbuild-language-server",
		match:          msbuild.Matches,
		build:          msbuild.BuildTools,
	},
	{
		name:           "nginx",
		language:       "Nginx",
		defaultCommand: "nginx-language-server",
		match:          nginx.Matches,
		build:          nginx.BuildTools,
	},
	{
		name:           "nim",
		language:       "Nim",
		defaultCommand: "nimlsp",
		match:          nim.Matches,
		build:          nim.BuildTools,
	},
	{
		name:           "nobl9yaml",
		language:       "Nobl9 YAML",
		defaultCommand: "nobl9-vscode",
		match:          nobl9yaml.Matches,
		build:          nobl9yaml.BuildTools,
	},
	{
		name:           "ocamlreason",
		language:       "OCaml/Reason",
		defaultCommand: "ocamllsp",
		match:          ocamlreason.Matches,
		build:          ocamlreason.BuildTools,
	},
	{
		name:           "odin",
		language:       "Odin",
		defaultCommand: "ols",
		match:          odin.Matches,
		build:          odin.BuildTools,
	},
	{
		name:           "openedgeabl",
		language:       "OpenEdge ABL",
		defaultCommand: "abl-language-server",
		match:          openedgeabl.Matches,
		build:          openedgeabl.BuildTools,
	},
	{
		name:           "openvalidation",
		language:       "openVALIDATION",
		defaultCommand: "ov-language-server",
		match:          openvalidation.Matches,
		build:          openvalidation.BuildTools,
	},
	{
		name:           "papyrus",
		language:       "Papyrus",
		defaultCommand: "papyrus-lang",
		match:          papyrus.Matches,
		build:          papyrus.BuildTools,
	},
	{
		name:           "partiql",
		language:       "PartiQL",
		defaultCommand: "aws-lsp-partiql",
		match:          partiql.Matches,
		build:          partiql.BuildTools,
	},
	{
		name:           "perl",
		language:       "Perl",
		defaultCommand: "perl-languageserver",
		match:          perl.Matches,
		build:          perl.BuildTools,
	},
	{
		name:           "pest",
		language:       "Pest",
		defaultCommand: "pest-ide-tools",
		match:          pest.Matches,
		build:          pest.BuildTools,
	},
	{
		name:           "pharo",
		language:       "Smalltalk/Pharo",
		defaultCommand: "pharolanguageserver",
		match:          pharo.Matches,
		build:          pharo.BuildTools,
	},
	{
		name:           "php",
		language:       "PHP",
		defaultCommand: "intelephense",
		match:          php.Matches,
		build:          php.BuildTools,
	},
	{
		name:           "phpunit",
		language:       "PHPUnit",
		defaultCommand: "phpunit-language-server",
		match:          phpunit.Matches,
		build:          phpunit.BuildTools,
	},
	{
		name:           "pli",
		language:       "IBM Enterprise PL/I for z/OS",
		defaultCommand: "pli-language-server",
		match:          pli.Matches,
		build:          pli.BuildTools,
	},
	{
		name:           "plsql",
		language:       "PL/SQL",
		defaultCommand: "plsql-language-server",
		match:          plsql.Matches,
		build:          plsql.BuildTools,
	},
	{
		name:           "polymer",
		language:       "Polymer",
		defaultCommand: "polymer-editor-service",
		match:          polymer.Matches,
		build:          polymer.BuildTools,
	},
	{
		name:           "powerpc",
		language:       "PowerPC Assembly",
		defaultCommand: "powerpc-support",
		match:          powerpc.Matches,
		build:          powerpc.BuildTools,
	},
	{
		name:           "powershell",
		language:       "PowerShell",
		defaultCommand: "powershell-editor-services",
		match:          powershell.Matches,
		build:          powershell.BuildTools,
	},
	{
		name:           "promql",
		language:       "PromQL",
		defaultCommand: "promql-langserver",
		match:          promql.Matches,
		build:          promql.BuildTools,
	},
	{
		name:           "protobuf",
		language:       "Protocol Buffers",
		defaultCommand: "protols",
		match:          protobuf.Matches,
		build:          protobuf.BuildTools,
	},
	{
		name:           "purescript",
		language:       "PureScript",
		defaultCommand: "purescript-language-server",
		match:          purescript.Matches,
		build:          purescript.BuildTools,
	},
	{
		name:           "puppet",
		language:       "Puppet",
		defaultCommand: "puppet-languageserver",
		match:          puppet.Matches,
		build:          puppet.BuildTools,
	},
	{
		name:           "python",
		language:       "Python",
		defaultCommand: "pyright-langserver",
		match:          python.Matches,
		build:          python.BuildTools,
	},
	{
		name:           "pony",
		language:       "Pony",
		defaultCommand: "ponyls",
		match:          pony.Matches,
		build:          pony.BuildTools,
	},
	{
		name:           "query",
		language:       "Query",
		defaultCommand: "ts_query_ls",
		match:          query.Matches,
		build:          query.BuildTools,
	},
	{
		name:           "qsharp",
		language:       "Q#",
		defaultCommand: "qsharp-language-server",
		match:          qsharp.Matches,
		build:          qsharp.BuildTools,
	},
	{
		name:           "racket",
		language:       "Racket",
		defaultCommand: "racket-langserver",
		match:          racket.Matches,
		build:          racket.BuildTools,
	},
	{
		name:           "rain",
		language:       "Rain",
		defaultCommand: "rainlanguageserver",
		match:          rain.Matches,
		build:          rain.BuildTools,
	},
	{
		name:           "raku",
		language:       "Raku",
		defaultCommand: "raku-navigator",
		match:          raku.Matches,
		build:          raku.BuildTools,
	},
	{
		name:           "raml",
		language:       "RAML",
		defaultCommand: "raml-language-server",
		match:          raml.Matches,
		build:          raml.BuildTools,
	},
	{
		name:           "rascal",
		language:       "Rascal",
		defaultCommand: "rascal-language-server",
		match:          rascal.Matches,
		build:          rascal.BuildTools,
	},
	{
		name:           "reasonml",
		language:       "ReasonML",
		defaultCommand: "reason-language-server",
		match:          reasonml.Matches,
		build:          reasonml.BuildTools,
	},
	{
		name:           "red",
		language:       "Red",
		defaultCommand: "redlangserver",
		match:          red.Matches,
		build:          red.BuildTools,
	},
	{
		name:           "rego",
		language:       "Rego",
		defaultCommand: "regal-language-server",
		match:          rego.Matches,
		build:          rego.BuildTools,
	},
	{
		name:           "rel",
		language:       "REL",
		defaultCommand: "rel-ls",
		match:          rel.Matches,
		build:          rel.BuildTools,
	},
	{
		name:           "rescript",
		language:       "ReScript",
		defaultCommand: "rescript-language-server",
		match:          rescript.Matches,
		build:          rescript.BuildTools,
	},
	{
		name:           "rexx",
		language:       "IBM TSO/E REXX",
		defaultCommand: "rexx-language-server",
		match:          rexx.Matches,
		build:          rexx.BuildTools,
	},
	{
		name:           "rlang",
		language:       "R",
		defaultCommand: "languageserver",
		match:          rlang.Matches,
		build:          rlang.BuildTools,
	},
	{
		name:           "robotframework",
		language:       "Robot Framework",
		defaultCommand: "robotframework-lsp",
		match:          robotframework.Matches,
		build:          robotframework.BuildTools,
	},
	{
		name:           "robotstxt",
		language:       "Robots.txt",
		defaultCommand: "robots-txt-language-server",
		match:          robotstxt.Matches,
		build:          robotstxt.BuildTools,
	},
	{
		name:           "ruby",
		language:       "Ruby",
		defaultCommand: "solargraph",
		match:          ruby.Matches,
		build:          ruby.BuildTools,
	},
	{
		name:           "rust",
		language:       "Rust",
		defaultCommand: "rust-analyzer",
		match:          rust.Matches,
		build:          rust.BuildTools,
	},
	{
		name:           "scala",
		language:       "Scala",
		defaultCommand: "metals",
		match:          scala.Matches,
		build:          scala.BuildTools,
	},
	{
		name:           "scheme",
		language:       "Scheme",
		defaultCommand: "scheme-langserver",
		match:          scheme.Matches,
		build:          scheme.BuildTools,
	},
	{
		name:           "shader",
		language:       "Shader",
		defaultCommand: "shader-language-server",
		match:          shader.Matches,
		build:          shader.BuildTools,
	},
	{
		name:           "slint",
		language:       "Slint",
		defaultCommand: "slint-lsp",
		match:          slint.Matches,
		build:          slint.BuildTools,
	},
	{
		name:           "smithy",
		language:       "Smithy",
		defaultCommand: "smithy-language-server",
		match:          smithy.Matches,
		build:          smithy.BuildTools,
	},
	{
		name:           "snyk",
		language:       "Snyk",
		defaultCommand: "snyk-ls",
		match:          snyk.Matches,
		build:          snyk.BuildTools,
	},
	{
		name:           "sparql",
		language:       "SPARQL",
		defaultCommand: "qlue-ls",
		match:          sparql.Matches,
		build:          sparql.BuildTools,
	},
	{
		name:           "sphinx",
		language:       "Sphinx",
		defaultCommand: "esbonio",
		match:          sphinx.Matches,
		build:          sphinx.BuildTools,
	},
	{
		name:           "sql",
		language:       "SQL",
		defaultCommand: "sqls",
		match:          sql.Matches,
		build:          sql.BuildTools,
	},
	{
		name:           "standardml",
		language:       "Standard ML",
		defaultCommand: "millet",
		match:          standardml.Matches,
		build:          standardml.BuildTools,
	},
	{
		name:           "stimulus",
		language:       "Stimulus",
		defaultCommand: "stimulus-language-server",
		match:          stimulus.Matches,
		build:          stimulus.BuildTools,
	},
	{
		name:           "stylable",
		language:       "Stylable",
		defaultCommand: "stylable-language-server",
		match:          stylable.Matches,
		build:          stylable.BuildTools,
	},
	{
		name:           "svelte",
		language:       "Svelte",
		defaultCommand: "svelteserver",
		match:          svelte.Matches,
		build:          svelte.BuildTools,
	},
	{
		name:           "sway",
		language:       "Sway",
		defaultCommand: "sway-lsp",
		match:          sway.Matches,
		build:          sway.BuildTools,
	},
	{
		name:           "swift",
		language:       "Swift",
		defaultCommand: "sourcekit-lsp",
		match:          swift.Matches,
		build:          swift.BuildTools,
	},
	{
		name:           "sysml2",
		language:       "SysML v2",
		defaultCommand: "sysml2-language-server",
		match:          sysml2.Matches,
		build:          sysml2.BuildTools,
	},
	{
		name:           "sysl",
		language:       "Sysl",
		defaultCommand: "sysl-language-server",
		match:          sysl.Matches,
		build:          sysl.BuildTools,
	},
	{
		name:           "systemd",
		language:       "systemd",
		defaultCommand: "systemd-language-server",
		match:          systemd.Matches,
		build:          systemd.BuildTools,
	},
	{
		name:           "systemtap",
		language:       "Systemtap",
		defaultCommand: "systemtap-language-server",
		match:          systemtap.Matches,
		build:          systemtap.BuildTools,
	},
	{
		name:           "systemverilog",
		language:       "SystemVerilog",
		defaultCommand: "svls",
		match:          systemverilog.Matches,
		build:          systemverilog.BuildTools,
	},
	{
		name:           "tsql",
		language:       "T-SQL",
		defaultCommand: "sqltoolsservice",
		match:          tsql.Matches,
		build:          tsql.BuildTools,
	},
	{
		name:           "tads3",
		language:       "Tads3",
		defaultCommand: "tads3-language-server",
		match:          tads3.Matches,
		build:          tads3.BuildTools,
	},
	{
		name:           "teal",
		language:       "Teal",
		defaultCommand: "teal-language-server",
		match:          teal.Matches,
		build:          teal.BuildTools,
	},
	{
		name:           "terraform",
		language:       "Terraform",
		defaultCommand: "terraform-ls",
		match:          terraform.Matches,
		build:          terraform.BuildTools,
	},
	{
		name:           "thrift",
		language:       "Thrift",
		defaultCommand: "thrift-ls",
		match:          thrift.Matches,
		build:          thrift.BuildTools,
	},
	{
		name:           "tibbobasic",
		language:       "Tibbo Basic",
		defaultCommand: "tibbo-basic-ls",
		match:          tibbobasic.Matches,
		build:          tibbobasic.BuildTools,
	},
	{
		name:           "toml",
		language:       "TOML",
		defaultCommand: "taplo",
		match:          toml.Matches,
		build:          toml.BuildTools,
	},
	{
		name:           "trinosql",
		language:       "Trino SQL",
		defaultCommand: "trinols",
		match:          trinosql.Matches,
		build:          trinosql.BuildTools,
	},
	{
		name:           "ttcn3",
		language:       "TTCN-3",
		defaultCommand: "ntt",
		match:          ttcn3.Matches,
		build:          ttcn3.BuildTools,
	},
	{
		name:           "turtle",
		language:       "Turtle",
		defaultCommand: "turtle-language-server",
		match:          turtle.Matches,
		build:          turtle.BuildTools,
	},
	{
		name:           "tailwindcss",
		language:       "Tailwind CSS",
		defaultCommand: "tailwindcss-language-server",
		match:          tailwindcss.Matches,
		build:          tailwindcss.BuildTools,
	},
	{
		name:           "twig",
		language:       "Twig",
		defaultCommand: "twig-language-server",
		match:          twig.Matches,
		build:          twig.BuildTools,
	},
	{
		name:           "typecobol",
		language:       "TypeCobol",
		defaultCommand: "typecobol-language-server",
		match:          typecobol.Matches,
		build:          typecobol.BuildTools,
	},
	{
		name:           "typescriptls",
		language:       "TypeScript",
		defaultCommand: "typescript-language-server",
		match:          typescriptls.Matches,
		build:          typescriptls.BuildTools,
	},
	{
		name:           "typst",
		language:       "Typst",
		defaultCommand: "tinymist",
		match:          typst.Matches,
		build:          typst.BuildTools,
	},
	{
		name:           "vlang",
		language:       "V",
		defaultCommand: "v-analyzer",
		match:          vlang.Matches,
		build:          vlang.BuildTools,
	},
	{
		name:           "vala",
		language:       "Vala",
		defaultCommand: "vala-language-server",
		match:          vala.Matches,
		build:          vala.BuildTools,
	},
	{
		name:           "vdm",
		language:       "VDM-SL, VDM++, VDM-RT",
		defaultCommand: "vdmj-lsp",
		match:          vdm.Matches,
		build:          vdm.BuildTools,
	},
	{
		name:           "veryl",
		language:       "Veryl",
		defaultCommand: "veryl-ls",
		match:          veryl.Matches,
		build:          veryl.BuildTools,
	},
	{
		name:           "vhdl",
		language:       "VHDL",
		defaultCommand: "vhdl_ls",
		match:          vhdl.Matches,
		build:          vhdl.BuildTools,
	},
	{
		name:           "viml",
		language:       "Viml",
		defaultCommand: "vim-language-server",
		match:          viml.Matches,
		build:          viml.BuildTools,
	},
	{
		name:           "visualforce",
		language:       "Visualforce",
		defaultCommand: "visualforce-language-server",
		match:          visualforce.Matches,
		build:          visualforce.BuildTools,
	},
	{
		name:           "vue",
		language:       "Vue",
		defaultCommand: "vls",
		match:          vue.Matches,
		build:          vue.BuildTools,
	},
	{
		name:           "wasm",
		language:       "WebAssembly",
		defaultCommand: "wasm-language-server",
		match:          wasm.Matches,
		build:          wasm.BuildTools,
	},
	{
		name:           "wgsl",
		language:       "WebGPU Shading Language",
		defaultCommand: "wgsl_analyzer",
		match:          wgsl.Matches,
		build:          wgsl.BuildTools,
	},
	{
		name:           "wikitext",
		language:       "Wikitext",
		defaultCommand: "wikitext-language-server",
		match:          wikitext.Matches,
		build:          wikitext.BuildTools,
	},
	{
		name:           "wing",
		language:       "Wing",
		defaultCommand: "wing-language-server",
		match:          wing.Matches,
		build:          wing.BuildTools,
	},
	{
		name:           "wolfram",
		language:       "Wolfram Language",
		defaultCommand: "lsp-wl",
		match:          wolfram.Matches,
		build:          wolfram.BuildTools,
	},
	{
		name:           "wxml",
		language:       "WXML",
		defaultCommand: "wxml-languageserver",
		match:          wxml.Matches,
		build:          wxml.BuildTools,
	},
	{
		name:           "xml",
		language:       "XML",
		defaultCommand: "xml-language-server",
		match:          xml.Matches,
		build:          xml.BuildTools,
	},
	{
		name:           "miniyaml",
		language:       "MiniYAML",
		defaultCommand: "miniyaml-language-server",
		match:          miniyaml.Matches,
		build:          miniyaml.BuildTools,
	},
	{
		name:           "yaml",
		language:       "YAML",
		defaultCommand: "yaml-language-server",
		match:          yaml.Matches,
		build:          yaml.BuildTools,
	},
	{
		name:           "yara",
		language:       "YARA",
		defaultCommand: "yara-language-server",
		match:          yara.Matches,
		build:          yara.BuildTools,
	},
	{
		name:           "yang",
		language:       "YANG",
		defaultCommand: "yang-lsp",
		match:          yang.Matches,
		build:          yang.BuildTools,
	},
	{
		name:           "zig",
		language:       "Zig",
		defaultCommand: "zls",
		match:          zig.Matches,
		build:          zig.BuildTools,
	},
	{
		name:           "nix",
		language:       "Nix",
		defaultCommand: "nil",
		match:          nix.Matches,
		build:          nix.BuildTools,
	},
	{
		name:           "efm",
		language:       "*",
		defaultCommand: "",
		match:          efm.Matches,
		build:          efm.BuildTools,
	},
	{
		name:           "diagnosticls",
		language:       "*",
		defaultCommand: "",
		match:          diagnosticls.Matches,
		build:          diagnosticls.BuildTools,
	},
	{
		name:           "tagls",
		language:       "*",
		defaultCommand: "",
		match:          tagls.Matches,
		build:          tagls.BuildTools,
	},
	{
		name:           "sonarlint",
		language:       "*",
		defaultCommand: "",
		match:          sonarlint.Matches,
		build:          sonarlint.BuildTools,
	},
	{
		name:           "testingls",
		language:       "*",
		defaultCommand: "",
		match:          testingls.Matches,
		build:          testingls.BuildTools,
	},
	{
		name:           "copilot",
		language:       "*",
		defaultCommand: "",
		match:          copilot.Matches,
		build:          copilot.BuildTools,
	},
	{
		name:           "harper",
		language:       "*",
		defaultCommand: "",
		match:          harper.Matches,
		build:          harper.BuildTools,
	},
	{
		name:           "sourcegraphgo",
		language:       "Go",
		defaultCommand: "go-langserver",
		match:          sourcegraphgo.Matches,
		build:          sourcegraphgo.BuildTools,
	},
	{
		name:           "gopls",
		language:       "Go",
		defaultCommand: "gopls",
		match:          gopls.Matches,
		build:          gopls.BuildTools,
	},
	{
		name:           "hlasm",
		language:       "IBM High Level Assembler",
		defaultCommand: "hlasm-language-server",
		match:          hlasm.Matches,
		build:          hlasm.BuildTools,
	},
	{
		name:           "ibmi",
		language:       "IBM i",
		defaultCommand: "ibmi-languages",
		match:          ibmi.Matches,
		build:          ibmi.BuildTools,
	},
	{
		name:           "qmlls",
		language:       "QML",
		defaultCommand: "qmlls",
		match:          qmlls.Matches,
		build:          qmlls.BuildTools,
	},
}

// BuildTools resolves and builds language server specific MCP tools.
func BuildTools(command string, args []string, client *lsp.Client, rootDir string) []mcp.Tool {
	var tools []mcp.Tool
	for _, spec := range matchedExtensionSpecs(command, args) {
		tools = append(tools, spec.build(client, rootDir)...)
	}
	return tools
}

// DescribeCommonToolScope returns a short explanation for generic LSP tools.
func DescribeCommonToolScope(command string, args []string) string {
	profiles := matchedProfileLabels(command, args)
	if len(profiles) == 0 {
		return "Language scope: generic LSP (no language-specific extension matched). Feature availability depends on server capabilities."
	}
	return "Language scope: " + strings.Join(profiles, ", ") + ". Feature availability depends on server capabilities."
}

func matchedExtensionSpecs(command string, args []string) []extensionSpec {
	var matched []extensionSpec
	for _, spec := range extensionSpecs {
		if spec.match(command, args) {
			matched = append(matched, spec)
		}
	}
	return matched
}

func matchedProfileLabels(command string, args []string) []string {
	matched := matchedExtensionSpecs(command, args)
	labels := make([]string, 0, len(matched))
	for _, spec := range matched {
		if spec.language == "" {
			labels = append(labels, spec.name)
			continue
		}
		labels = append(labels, spec.language+" ("+spec.name+")")
	}
	return labels
}

// ExtensionMeta は LSP 拡張を MCP Definition として登録するためのメタデータ。
type ExtensionMeta struct {
	Name           string
	Language       string
	DefaultCommand string
}

// AllExtensionMeta は defaultCommand を持つ言語固有の拡張メタデータを返す。
// language="*" のジェネリックツールと defaultCommand 未設定のエントリは除外。
func AllExtensionMeta() []ExtensionMeta {
	var result []ExtensionMeta
	for _, spec := range extensionSpecs {
		if spec.language == "*" || spec.defaultCommand == "" {
			continue
		}
		result = append(result, ExtensionMeta{
			Name:           spec.name,
			Language:       spec.language,
			DefaultCommand: spec.defaultCommand,
		})
	}
	return result
}
