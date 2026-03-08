# generic-lsp-mcp (Go)

Go実装の汎用LSP向けMCPサーバーです。  
利用者向けガイドと開発者向けガイドを分離しています。

## ドキュメント

1. 利用者向け: `USER_GUIDE.md`
2. 開発者向け: `DEVELOPER_GUIDE.md`

## クイック参照

- 実行バイナリ: `generic-lsp-mcp.exe`（Windows）
- エントリーポイント: `cmd/generic-lsp-mcp/main.go`
- OSS向け公開API: `app.go`（`package lspmcp`）
- ツール定義: `internal/tools/registry.go`

## Go API（外部呼び出し）

このモジュールはCLI実行に加えて、Goコードから直接呼び出せます。

```go
runtime, err := lspmcp.NewApp(lspmcp.Config{
    LSPCommand: "gopls",
    RootDir:    ".",
})
if err != nil {
    // handle error
}
if err := runtime.Serve(ctx); err != nil {
    // handle error
}
```

またはワンショットで:

```go
err := lspmcp.Run(ctx, lspmcp.Config{
    LSPCommand: "gopls",
    RootDir:    ".",
})
```

## 実装・動作確認ステータス

### 詳細実装済み一覧

| 区分 | 実装内容（詳細） | 実装状態 |
| --- | --- | --- |
| 共通LSPツール | `lsp_check_capabilities`, `lsp_get_hover`, `lsp_get_definitions`, `lsp_get_declarations`, `lsp_get_type_definitions`, `lsp_get_implementations`, `lsp_find_references`, `lsp_get_document_symbols`, `lsp_get_workspace_symbols`, `lsp_resolve_workspace_symbol`, `lsp_get_completion`, `lsp_resolve_completion_item`, `lsp_get_signature_help`, `lsp_get_diagnostics`, `lsp_get_workspace_diagnostics`, `lsp_get_code_actions`, `lsp_resolve_code_action`, `lsp_format_document`, `lsp_format_range`, `lsp_format_on_type`, `lsp_rename_symbol`, `lsp_prepare_rename`, `lsp_execute_command` | 実装済み |
| abap拡張ツール（実装例） | `abap_list_extension_commands`, `abap_execute_extension_command` | 実装済み |
| as2拡張ツール（実装例） | `as2_list_extension_commands`, `as2_execute_extension_command` | 実装済み |
| asn1拡張ツール（実装例） | `asn1_list_extension_commands`, `asn1_execute_extension_command` | 実装済み |
| ada拡張ツール（実装例） | `ada_list_extension_commands`, `ada_execute_extension_command` | 実装済み |
| agda拡張ツール（実装例） | `agda_list_extension_commands`, `agda_execute_extension_command` | 実装済み |
| aml拡張ツール（実装例） | `aml_list_extension_commands`, `aml_execute_extension_command`（AML / AsyncAPI / OpenAPI / RAML 共通） | 実装済み |
| ansible拡張ツール（実装例） | `ansible_list_extension_commands`, `ansible_execute_extension_command` | 実装済み |
| angular拡張ツール（実装例） | `angular_list_extension_commands`, `angular_execute_extension_command` | 実装済み |
| antlr拡張ツール（実装例） | `antlr_list_extension_commands`, `antlr_execute_extension_command` | 実装済み |
| apielements拡張ツール（実装例） | `apielements_list_extension_commands`, `apielements_execute_extension_command` | 実装済み |
| apl拡張ツール（実装例） | `apl_list_extension_commands`, `apl_execute_extension_command` | 実装済み |
| camel拡張ツール（実装例） | `camel_list_extension_commands`, `camel_execute_extension_command` | 実装済み |
| apachedispatcher拡張ツール（実装例） | `apachedispatcher_list_extension_commands`, `apachedispatcher_execute_extension_command` | 実装済み |
| apex拡張ツール（実装例） | `apex_list_extension_commands`, `apex_execute_extension_command` | 実装済み |
| astro拡張ツール（実装例） | `astro_list_extension_commands`, `astro_execute_extension_command` | 実装済み |
| awk拡張ツール（実装例） | `awk_list_extension_commands`, `awk_execute_extension_command` | 実装済み |
| bake拡張ツール（実装例） | `bake_list_extension_commands`, `bake_execute_extension_command`（Bake / Compose / Dockerfiles 共通） | 実装済み |
| ballerina拡張ツール（実装例） | `ballerina_list_extension_commands`, `ballerina_execute_extension_command` | 実装済み |
| bash拡張ツール（実装例） | `bash_list_extension_commands`, `bash_execute_extension_command` | 実装済み |
| batch拡張ツール（実装例） | `batch_list_extension_commands`, `batch_execute_extension_command` | 実装済み |
| bazel拡張ツール（実装例） | `bazel_list_extension_commands`, `bazel_execute_extension_command` | 実装済み |
| bicep拡張ツール（実装例） | `bicep_list_extension_commands`, `bicep_execute_extension_command` | 実装済み |
| bitbake拡張ツール（実装例） | `bitbake_list_extension_commands`, `bitbake_execute_extension_command` | 実装済み |
| bsl拡張ツール（実装例） | `bsl_list_extension_commands`, `bsl_execute_extension_command` | 実装済み |
| boriel拡張ツール（実装例） | `boriel_list_extension_commands`, `boriel_execute_extension_command` | 実装済み |
| brighterscript拡張ツール（実装例） | `brighterscript_list_extension_commands`, `brighterscript_execute_extension_command` | 実装済み |
| bprob拡張ツール（実装例） | `bprob_list_extension_commands`, `bprob_execute_extension_command` | 実装済み |
| caddy拡張ツール（実装例） | `caddy_list_extension_commands`, `caddy_execute_extension_command` | 実装済み |
| cds拡張ツール（実装例） | `cds_list_extension_commands`, `cds_execute_extension_command` | 実装済み |
| cssls拡張ツール（実装例） | `cssls_list_extension_commands`, `cssls_execute_extension_command` | 実装済み |
| ceylon拡張ツール（実装例） | `ceylon_list_extension_commands`, `ceylon_execute_extension_command` | 実装済み |
| clarity拡張ツール（実装例） | `clarity_list_extension_commands`, `clarity_execute_extension_command` | 実装済み |
| clojure拡張ツール（実装例） | `clojure_list_extension_commands`, `clojure_execute_extension_command` | 実装済み |
| cmake拡張ツール（実装例） | `cmake_list_extension_commands`, `cmake_execute_extension_command`（cmake-language-server / neocmakelsp 共通） | 実装済み |
| commonlisp拡張ツール（実装例） | `commonlisp_list_extension_commands`, `commonlisp_execute_extension_command` | 実装済み |
| chapel拡張ツール（実装例） | `chapel_list_extension_commands`, `chapel_execute_extension_command` | 実装済み |
| coq拡張ツール（実装例） | `coq_list_extension_commands`, `coq_execute_extension_command`（coq-lsp / vscoq 共通） | 実装済み |
| cobol拡張ツール（実装例） | `cobol_list_extension_commands`, `cobol_execute_extension_command`（rech-editor-cobol / cobol-language-support 共通） | 実装済み |
| codeql拡張ツール（実装例） | `codeql_list_extension_commands`, `codeql_execute_extension_command` | 実装済み |
| coffeescript拡張ツール（実装例） | `coffeescript_list_extension_commands`, `coffeescript_execute_extension_command` | 実装済み |
| crystal拡張ツール（実装例） | `crystal_list_extension_commands`, `crystal_execute_extension_command`（crystalline / scry 共通） | 実装済み |
| cwl拡張ツール（実装例） | `cwl_list_extension_commands`, `cwl_execute_extension_command` | 実装済み |
| cucumber拡張ツール（実装例） | `cucumber_list_extension_commands`, `cucumber_execute_extension_command` | 実装済み |
| cython拡張ツール（実装例） | `cython_list_extension_commands`, `cython_execute_extension_command` | 実装済み |
| dlang拡張ツール（実装例） | `dlang_list_extension_commands`, `dlang_execute_extension_command`（serve-d / dls 共通） | 実装済み |
| dart拡張ツール（実装例） | `dart_list_extension_commands`, `dart_execute_extension_command` | 実装済み |
| datapack拡張ツール（実装例） | `datapack_list_extension_commands`, `datapack_execute_extension_command` | 実装済み |
| debian拡張ツール（実装例） | `debian_list_extension_commands`, `debian_execute_extension_command` | 実装済み |
| delphi拡張ツール（実装例） | `delphi_list_extension_commands`, `delphi_execute_extension_command` | 実装済み |
| denizenscript拡張ツール（実装例） | `denizenscript_list_extension_commands`, `denizenscript_execute_extension_command` | 実装済み |
| devicetree拡張ツール（実装例） | `devicetree_list_extension_commands`, `devicetree_execute_extension_command` | 実装済み |
| deno拡張ツール（実装例） | `deno_list_extension_commands`, `deno_execute_extension_command` | 実装済み |
| dockerfile拡張ツール（実装例） | `dockerfile_list_extension_commands`, `dockerfile_execute_extension_command` | 実装済み |
| dreammaker拡張ツール（実装例） | `dreammaker_list_extension_commands`, `dreammaker_execute_extension_command` | 実装済み |
| egglog拡張ツール（実装例） | `egglog_list_extension_commands`, `egglog_execute_extension_command` | 実装済み |
| emacslisp拡張ツール（実装例） | `emacslisp_list_extension_commands`, `emacslisp_execute_extension_command` | 実装済み |
| erlang拡張ツール（実装例） | `erlang_list_extension_commands`, `erlang_execute_extension_command`（sourcer / erlang_ls / ELP 共通） | 実装済み |
| erg拡張ツール（実装例） | `erg_list_extension_commands`, `erg_execute_extension_command` | 実装済み |
| elixir拡張ツール（実装例） | `elixir_list_extension_commands`, `elixir_execute_extension_command` | 実装済み |
| elm拡張ツール（実装例） | `elm_list_extension_commands`, `elm_execute_extension_command` | 実装済み |
| ember拡張ツール（実装例） | `ember_list_extension_commands`, `ember_execute_extension_command`（lifeart / ember-watch 共通） | 実装済み |
| fsharp拡張ツール（実装例） | `fsharp_list_extension_commands`, `fsharp_execute_extension_command`（F# Language Server / FsAutoComplete 共通） | 実装済み |
| fish拡張ツール（実装例） | `fish_list_extension_commands`, `fish_execute_extension_command` | 実装済み |
| fluentbit拡張ツール（実装例） | `fluentbit_list_extension_commands`, `fluentbit_execute_extension_command` | 実装済み |
| fortran拡張ツール（実装例） | `fortran_list_extension_commands`, `fortran_execute_extension_command`（fortran-language-server / fortls 共通） | 実装済み |
| fuzion拡張ツール（実装例） | `fuzion_list_extension_commands`, `fuzion_execute_extension_command` | 実装済み |
| glsl拡張ツール（実装例） | `glsl_list_extension_commands`, `glsl_execute_extension_command` | 実装済み |
| mcshader拡張ツール（実装例） | `mcshader_list_extension_commands`, `mcshader_execute_extension_command` | 実装済み |
| gauge拡張ツール（実装例） | `gauge_list_extension_commands`, `gauge_execute_extension_command` | 実装済み |
| gdscript拡張ツール（実装例） | `gdscript_list_extension_commands`, `gdscript_execute_extension_command` | 実装済み |
| gleam拡張ツール（実装例） | `gleam_list_extension_commands`, `gleam_execute_extension_command` | 実装済み |
| glimmer拡張ツール（実装例） | `glimmer_list_extension_commands`, `glimmer_execute_extension_command` | 実装済み |
| gluon拡張ツール（実装例） | `gluon_list_extension_commands`, `gluon_execute_extension_command` | 実装済み |
| gn拡張ツール（実装例） | `gn_list_extension_commands`, `gn_execute_extension_command` | 実装済み |
| sourcegraphgo拡張ツール（実装例） | `sourcegraphgo_list_extension_commands`, `sourcegraphgo_execute_extension_command` | 実装済み |
| graphql拡張ツール（実装例） | `graphql_list_extension_commands`, `graphql_execute_extension_command`（Official GraphQL Language Server / GQL Language Server 共通） | 実装済み |
| dot拡張ツール（実装例） | `dot_list_extension_commands`, `dot_execute_extension_command` | 実装済み |
| grain拡張ツール（実装例） | `grain_list_extension_commands`, `grain_execute_extension_command` | 実装済み |
| groovy拡張ツール（実装例） | `groovy_list_extension_commands`, `groovy_execute_extension_command`（palantir / Prominic / vscode-groovy-lint 共通） | 実装済み |
| html拡張ツール（実装例） | `html_list_extension_commands`, `html_execute_extension_command`（vscode-html-languageserver / SuperHTML 共通） | 実装済み |
| haskell拡張ツール（実装例） | `haskell_list_extension_commands`, `haskell_execute_extension_command` | 実装済み |
| haxe拡張ツール（実装例） | `haxe_list_extension_commands`, `haxe_execute_extension_command` | 実装済み |
| helm拡張ツール（実装例） | `helm_list_extension_commands`, `helm_execute_extension_command` | 実装済み |
| hlsl拡張ツール（実装例） | `hlsl_list_extension_commands`, `hlsl_execute_extension_command` | 実装済み |
| ink拡張ツール（実装例） | `ink_list_extension_commands`, `ink_execute_extension_command` | 実装済み |
| isabelle拡張ツール（実装例） | `isabelle_list_extension_commands`, `isabelle_execute_extension_command` | 実装済み |
| idris2拡張ツール（実装例） | `idris2_list_extension_commands`, `idris2_execute_extension_command` | 実装済み |
| java拡張ツール（実装例） | `java_list_extension_commands`, `java_execute_extension_command`（Eclipse JDT LS / java-language-server 共通） | 実装済み |
| javascript拡張ツール（実装例） | `javascript_list_extension_commands`, `javascript_execute_extension_command` | 実装済み |
| flow拡張ツール（実装例） | `flow_list_extension_commands`, `flow_execute_extension_command`（flow / flow-language-server 共通） | 実装済み |
| jstypescript拡張ツール（実装例） | `jstypescript_list_extension_commands`, `jstypescript_execute_extension_command`（sourcegraph javascript-typescript / biome_lsp 共通） | 実装済み |
| jcl拡張ツール（実装例） | `jcl_list_extension_commands`, `jcl_execute_extension_command` | 実装済み |
| jimmerdto拡張ツール（実装例） | `jimmerdto_list_extension_commands`, `jimmerdto_execute_extension_command` | 実装済み |
| jsonls拡張ツール（実装例） | `jsonls_list_extension_commands`, `jsonls_execute_extension_command` | 実装済み |
| jsonnet拡張ツール（実装例） | `jsonnet_list_extension_commands`, `jsonnet_execute_extension_command` | 実装済み |
| julia拡張ツール（実装例） | `julia_list_extension_commands`, `julia_execute_extension_command` | 実装済み |
| kconfig拡張ツール（実装例） | `kconfig_list_extension_commands`, `kconfig_execute_extension_command` | 実装済み |
| kdl拡張ツール（実装例） | `kdl_list_extension_commands`, `kdl_execute_extension_command` | 実装済み |
| kedro拡張ツール（実装例） | `kedro_list_extension_commands`, `kedro_execute_extension_command` | 実装済み |
| kerboscript拡張ツール（実装例） | `kerboscript_list_extension_commands`, `kerboscript_execute_extension_command` | 実装済み |
| kerml拡張ツール（実装例） | `kerml_list_extension_commands`, `kerml_execute_extension_command` | 実装済み |
| kotlin拡張ツール（実装例） | `kotlin_list_extension_commands`, `kotlin_execute_extension_command`（kotlin-language-server / kotlin-lsp 共通） | 実装済み |
| typecobolrobot拡張ツール（実装例） | `typecobolrobot_list_extension_commands`, `typecobolrobot_execute_extension_command` | 実装済み |
| languagetool拡張ツール（実装例） | `languagetool_list_extension_commands`, `languagetool_execute_extension_command`（languagetool-languageserver / ltex-ls 共通） | 実装済み |
| lark拡張ツール（実装例） | `lark_list_extension_commands`, `lark_execute_extension_command` | 実装済み |
| latex拡張ツール（実装例） | `latex_list_extension_commands`, `latex_execute_extension_command` | 実装済み |
| lean4拡張ツール（実装例） | `lean4_list_extension_commands`, `lean4_execute_extension_command` | 実装済み |
| lox拡張ツール（実装例） | `lox_list_extension_commands`, `lox_execute_extension_command` | 実装済み |
| lpc拡張ツール（実装例） | `lpc_list_extension_commands`, `lpc_execute_extension_command` | 実装済み |
| lua拡張ツール（実装例） | `lua_list_extension_commands`, `lua_execute_extension_command`（lua-lsp / lua-language-server / LuaHelper 共通） | 実装済み |
| liquid拡張ツール（実装例） | `liquid_list_extension_commands`, `liquid_execute_extension_command` | 実装済み |
| lpg拡張ツール（実装例） | `lpg_list_extension_commands`, `lpg_execute_extension_command` | 実装済み |
| make拡張ツール（実装例） | `make_list_extension_commands`, `make_execute_extension_command` | 実装済み |
| markdown拡張ツール（実装例） | `markdown_list_extension_commands`, `markdown_execute_extension_command`（Marksman / Markmark / vscode-markdown-languageserver 共通） | 実装済み |
| matlab拡張ツール（実装例） | `matlab_list_extension_commands`, `matlab_execute_extension_command` | 実装済み |
| mdx拡張ツール（実装例） | `mdx_list_extension_commands`, `mdx_execute_extension_command` | 実装済み |
| m68k拡張ツール（実装例） | `m68k_list_extension_commands`, `m68k_execute_extension_command` | 実装済み |
| msbuild拡張ツール（実装例） | `msbuild_list_extension_commands`, `msbuild_execute_extension_command` | 実装済み |
| asmlsp拡張ツール（実装例） | `asmlsp_list_extension_commands`, `asmlsp_execute_extension_command` | 実装済み |
| nginx拡張ツール（実装例） | `nginx_list_extension_commands`, `nginx_execute_extension_command` | 実装済み |
| nim拡張ツール（実装例） | `nim_list_extension_commands`, `nim_execute_extension_command` | 実装済み |
| nobl9yaml拡張ツール（実装例） | `nobl9yaml_list_extension_commands`, `nobl9yaml_execute_extension_command` | 実装済み |
| ocamlreason拡張ツール（実装例） | `ocamlreason_list_extension_commands`, `ocamlreason_execute_extension_command` | 実装済み |
| odin拡張ツール（実装例） | `odin_list_extension_commands`, `odin_execute_extension_command` | 実装済み |
| openedgeabl拡張ツール（実装例） | `openedgeabl_list_extension_commands`, `openedgeabl_execute_extension_command` | 実装済み |
| openvalidation拡張ツール（実装例） | `openvalidation_list_extension_commands`, `openvalidation_execute_extension_command` | 実装済み |
| papyrus拡張ツール（実装例） | `papyrus_list_extension_commands`, `papyrus_execute_extension_command` | 実装済み |
| partiql拡張ツール（実装例） | `partiql_list_extension_commands`, `partiql_execute_extension_command` | 実装済み |
| perl拡張ツール（実装例） | `perl_list_extension_commands`, `perl_execute_extension_command`（Perl::LanguageServer / PLS / Perl Navigator 共通） | 実装済み |
| pest拡張ツール（実装例） | `pest_list_extension_commands`, `pest_execute_extension_command` | 実装済み |
| php拡張ツール（実装例） | `php_list_extension_commands`, `php_execute_extension_command`（Crane / intelephense / php-language-server / Serenata / Phan / phpactor 共通） | 実装済み |
| phpunit拡張ツール（実装例） | `phpunit_list_extension_commands`, `phpunit_execute_extension_command` | 実装済み |
| pli拡張ツール（実装例） | `pli_list_extension_commands`, `pli_execute_extension_command` | 実装済み |
| plsql拡張ツール（実装例） | `plsql_list_extension_commands`, `plsql_execute_extension_command` | 実装済み |
| polymer拡張ツール（実装例） | `polymer_list_extension_commands`, `polymer_execute_extension_command` | 実装済み |
| powerpc拡張ツール（実装例） | `powerpc_list_extension_commands`, `powerpc_execute_extension_command` | 実装済み |
| powershell拡張ツール（実装例） | `powershell_list_extension_commands`, `powershell_execute_extension_command` | 実装済み |
| promql拡張ツール（実装例） | `promql_list_extension_commands`, `promql_execute_extension_command` | 実装済み |
| protobuf拡張ツール（実装例） | `protobuf_list_extension_commands`, `protobuf_execute_extension_command`（protols / protobuf-language-server / buf lsp 共通） | 実装済み |
| purescript拡張ツール（実装例） | `purescript_list_extension_commands`, `purescript_execute_extension_command` | 実装済み |
| puppet拡張ツール（実装例） | `puppet_list_extension_commands`, `puppet_execute_extension_command` | 実装済み |
| python拡張ツール（実装例） | `python_list_extension_commands`, `python_execute_extension_command`（ty / PyDev / Pyright / Pyrefly / basedpyright / pylsp / jedi-language-server / pylyzer / zuban 共通） | 実装済み |
| pony拡張ツール（実装例） | `pony_list_extension_commands`, `pony_execute_extension_command` | 実装済み |
| qsharp拡張ツール（実装例） | `qsharp_list_extension_commands`, `qsharp_execute_extension_command` | 実装済み |
| query拡張ツール（実装例） | `query_list_extension_commands`, `query_execute_extension_command` | 実装済み |
| rlang拡張ツール（実装例） | `rlang_list_extension_commands`, `rlang_execute_extension_command` | 実装済み |
| racket拡張ツール（実装例） | `racket_list_extension_commands`, `racket_execute_extension_command` | 実装済み |
| rain拡張ツール（実装例） | `rain_list_extension_commands`, `rain_execute_extension_command` | 実装済み |
| raku拡張ツール（実装例） | `raku_list_extension_commands`, `raku_execute_extension_command` | 実装済み |
| raml拡張ツール（実装例） | `raml_list_extension_commands`, `raml_execute_extension_command` | 実装済み |
| rascal拡張ツール（実装例） | `rascal_list_extension_commands`, `rascal_execute_extension_command` | 実装済み |
| reasonml拡張ツール（実装例） | `reasonml_list_extension_commands`, `reasonml_execute_extension_command` | 実装済み |
| red拡張ツール（実装例） | `red_list_extension_commands`, `red_execute_extension_command` | 実装済み |
| rego拡張ツール（実装例） | `rego_list_extension_commands`, `rego_execute_extension_command` | 実装済み |
| rel拡張ツール（実装例） | `rel_list_extension_commands`, `rel_execute_extension_command` | 実装済み |
| rescript拡張ツール（実装例） | `rescript_list_extension_commands`, `rescript_execute_extension_command` | 実装済み |
| rexx拡張ツール（実装例） | `rexx_list_extension_commands`, `rexx_execute_extension_command` | 実装済み |
| robotframework拡張ツール（実装例） | `robotframework_list_extension_commands`, `robotframework_execute_extension_command`（RobotCode / robotframework-lsp 共通） | 実装済み |
| robotstxt拡張ツール（実装例） | `robotstxt_list_extension_commands`, `robotstxt_execute_extension_command` | 実装済み |
| ruby拡張ツール（実装例） | `ruby_list_extension_commands`, `ruby_execute_extension_command`（solargraph / language_server-ruby / sorbet / orbacle / ruby_language_server / ruby-lsp 共通） | 実装済み |
| rust拡張ツール（実装例） | `rust_list_extension_commands`, `rust_execute_extension_command` | 実装済み |
| scala拡張ツール（実装例） | `scala_list_extension_commands`, `scala_execute_extension_command`（dragos-vscode-scala / Metals 共通） | 実装済み |
| scheme拡張ツール（実装例） | `scheme_list_extension_commands`, `scheme_execute_extension_command` | 実装済み |
| shader拡張ツール（実装例） | `shader_list_extension_commands`, `shader_execute_extension_command` | 実装済み |
| slint拡張ツール（実装例） | `slint_list_extension_commands`, `slint_execute_extension_command` | 実装済み |
| pharo拡張ツール（実装例） | `pharo_list_extension_commands`, `pharo_execute_extension_command` | 実装済み |
| smithy拡張ツール（実装例） | `smithy_list_extension_commands`, `smithy_execute_extension_command` | 実装済み |
| snyk拡張ツール（実装例） | `snyk_list_extension_commands`, `snyk_execute_extension_command` | 実装済み |
| sparql拡張ツール（実装例） | `sparql_list_extension_commands`, `sparql_execute_extension_command`（Qlue-ls / SPARQL Language Server 共通） | 実装済み |
| sphinx拡張ツール（実装例） | `sphinx_list_extension_commands`, `sphinx_execute_extension_command` | 実装済み |
| sql拡張ツール（実装例） | `sql_list_extension_commands`, `sql_execute_extension_command` | 実装済み |
| standardml拡張ツール（実装例） | `standardml_list_extension_commands`, `standardml_execute_extension_command` | 実装済み |
| stimulus拡張ツール（実装例） | `stimulus_list_extension_commands`, `stimulus_execute_extension_command` | 実装済み |
| stylable拡張ツール（実装例） | `stylable_list_extension_commands`, `stylable_execute_extension_command` | 実装済み |
| svelte拡張ツール（実装例） | `svelte_list_extension_commands`, `svelte_execute_extension_command` | 実装済み |
| sway拡張ツール（実装例） | `sway_list_extension_commands`, `sway_execute_extension_command` | 実装済み |
| swift拡張ツール（実装例） | `swift_list_extension_commands`, `swift_execute_extension_command` | 実装済み |
| sysml2拡張ツール（実装例） | `sysml2_list_extension_commands`, `sysml2_execute_extension_command` | 実装済み |
| sysl拡張ツール（実装例） | `sysl_list_extension_commands`, `sysl_execute_extension_command` | 実装済み |
| systemd拡張ツール（実装例） | `systemd_list_extension_commands`, `systemd_execute_extension_command` | 実装済み |
| systemtap拡張ツール（実装例） | `systemtap_list_extension_commands`, `systemtap_execute_extension_command` | 実装済み |
| systemverilog拡張ツール（実装例） | `systemverilog_list_extension_commands`, `systemverilog_execute_extension_command`（svls / Sigasi / Verible / slang-server 共通） | 実装済み |
| tsql拡張ツール（実装例） | `tsql_list_extension_commands`, `tsql_execute_extension_command` | 実装済み |
| tads3拡張ツール（実装例） | `tads3_list_extension_commands`, `tads3_execute_extension_command` | 実装済み |
| teal拡張ツール（実装例） | `teal_list_extension_commands`, `teal_execute_extension_command` | 実装済み |
| terraform拡張ツール（実装例） | `terraform_list_extension_commands`, `terraform_execute_extension_command`（terraform-lsp / terraform-ls 共通） | 実装済み |
| thrift拡張ツール（実装例） | `thrift_list_extension_commands`, `thrift_execute_extension_command`（software-mansion/ocfbnj thrift-ls 共通） | 実装済み |
| tibbobasic拡張ツール（実装例） | `tibbobasic_list_extension_commands`, `tibbobasic_execute_extension_command` | 実装済み |
| toml拡張ツール（実装例） | `toml_list_extension_commands`, `toml_execute_extension_command`（Taplo / Tombi 共通） | 実装済み |
| trinosql拡張ツール（実装例） | `trinosql_list_extension_commands`, `trinosql_execute_extension_command` | 実装済み |
| ttcn3拡張ツール（実装例） | `ttcn3_list_extension_commands`, `ttcn3_execute_extension_command`（ntt / Titan Language Server 共通） | 実装済み |
| turtle拡張ツール（実装例） | `turtle_list_extension_commands`, `turtle_execute_extension_command` | 実装済み |
| tailwindcss拡張ツール（実装例） | `tailwindcss_list_extension_commands`, `tailwindcss_execute_extension_command` | 実装済み |
| twig拡張ツール（実装例） | `twig_list_extension_commands`, `twig_execute_extension_command` | 実装済み |
| typecobol拡張ツール（実装例） | `typecobol_list_extension_commands`, `typecobol_execute_extension_command` | 実装済み |
| typescriptls拡張ツール（実装例） | `typescriptls_list_extension_commands`, `typescriptls_execute_extension_command` | 実装済み |
| typst拡張ツール（実装例） | `typst_list_extension_commands`, `typst_execute_extension_command`（tinymist / typst-lsp 共通） | 実装済み |
| vlang拡張ツール（実装例） | `vlang_list_extension_commands`, `vlang_execute_extension_command` | 実装済み |
| vala拡張ツール（実装例） | `vala_list_extension_commands`, `vala_execute_extension_command` | 実装済み |
| vdm拡張ツール（実装例） | `vdm_list_extension_commands`, `vdm_execute_extension_command` | 実装済み |
| veryl拡張ツール（実装例） | `veryl_list_extension_commands`, `veryl_execute_extension_command` | 実装済み |
| vhdl拡張ツール（実装例） | `vhdl_list_extension_commands`, `vhdl_execute_extension_command`（vhdl_ls / Sigasi / VHDL for Professionals 共通） | 実装済み |
| viml拡張ツール（実装例） | `viml_list_extension_commands`, `viml_execute_extension_command` | 実装済み |
| visualforce拡張ツール（実装例） | `visualforce_list_extension_commands`, `visualforce_execute_extension_command` | 実装済み |
| vue拡張ツール（実装例） | `vue_list_extension_commands`, `vue_execute_extension_command`（vetur / language-tools 共通） | 実装済み |
| wasm拡張ツール（実装例） | `wasm_list_extension_commands`, `wasm_execute_extension_command`（wasm-language-tools / wasm-language-server 共通） | 実装済み |
| wgsl拡張ツール（実装例） | `wgsl_list_extension_commands`, `wgsl_execute_extension_command` | 実装済み |
| wikitext拡張ツール（実装例） | `wikitext_list_extension_commands`, `wikitext_execute_extension_command` | 実装済み |
| wing拡張ツール（実装例） | `wing_list_extension_commands`, `wing_execute_extension_command` | 実装済み |
| wolfram拡張ツール（実装例） | `wolfram_list_extension_commands`, `wolfram_execute_extension_command`（lsp-wl / LSPServer / wlsp 共通） | 実装済み |
| wxml拡張ツール（実装例） | `wxml_list_extension_commands`, `wxml_execute_extension_command` | 実装済み |
| xml拡張ツール（実装例） | `xml_list_extension_commands`, `xml_execute_extension_command`（IBM XML Language Server / LemMinX 共通） | 実装済み |
| miniyaml拡張ツール（実装例） | `miniyaml_list_extension_commands`, `miniyaml_execute_extension_command` | 実装済み |
| yaml拡張ツール（実装例） | `yaml_list_extension_commands`, `yaml_execute_extension_command`（vscode-yaml-languageservice / yaml-language-server 共通） | 実装済み |
| yara拡張ツール（実装例） | `yara_list_extension_commands`, `yara_execute_extension_command` | 実装済み |
| yang拡張ツール（実装例） | `yang_list_extension_commands`, `yang_execute_extension_command` | 実装済み |
| zig拡張ツール（実装例） | `zig_list_extension_commands`, `zig_execute_extension_command` | 実装済み |
| nix拡張ツール（実装例） | `nix_list_extension_commands`, `nix_execute_extension_command`（nil / nixd 共通） | 実装済み |
| efm拡張ツール（実装例） | `efm_list_extension_commands`, `efm_execute_extension_command` | 実装済み |
| diagnosticls拡張ツール（実装例） | `diagnosticls_list_extension_commands`, `diagnosticls_execute_extension_command` | 実装済み |
| tagls拡張ツール（実装例） | `tagls_list_extension_commands`, `tagls_execute_extension_command` | 実装済み |
| sonarlint拡張ツール（実装例） | `sonarlint_list_extension_commands`, `sonarlint_execute_extension_command` | 実装済み |
| testingls拡張ツール（実装例） | `testingls_list_extension_commands`, `testingls_execute_extension_command` | 実装済み |
| copilot拡張ツール（実装例） | `copilot_list_extension_commands`, `copilot_execute_extension_command` | 実装済み |
| harper拡張ツール（実装例） | `harper_list_extension_commands`, `harper_execute_extension_command` | 実装済み |
| gopls拡張ツール（実装例） | `gopls_list_extension_commands`, `gopls_execute_command` | 実装済み |
| hlasm拡張ツール（実装例） | `hlasm_list_extension_commands`, `hlasm_execute_extension_command` | 実装済み |
| ibmi拡張ツール（実装例） | `ibmi_list_extension_commands`, `ibmi_execute_extension_command`（IBM i RPG/CL 共通） | 実装済み |
| clangd拡張ツール（実装例） | `clangd_switch_source_header`, `clangd_get_symbol_info` | 実装済み |
| ccls拡張ツール（実装例） | `ccls_get_call_hierarchy`, `ccls_get_inheritance_hierarchy`, `ccls_get_member_hierarchy`, `ccls_get_vars`, `ccls_navigate` | 実装済み |
| csharp拡張ツール（実装例） | `csharp_list_extension_commands`, `csharp_execute_extension_command`（OmniSharp / LanguageServer.NET / razzmatazz-csharp-language-server(`csharp-ls`) 系） | 実装済み |
| cquery拡張ツール（実装例） | `cquery_get_base`, `cquery_get_derived`, `cquery_get_callers`, `cquery_get_vars` | 実装済み |
| cpptools拡張ツール（実装例） | `cpptools_list_extension_methods`, `cpptools_call_extension_method`, `cpptools_get_includes`, `cpptools_switch_header_source` | 実装済み |
| qmlls拡張ツール（実装例） | `qmlls_list_extension_commands`, `qmlls_execute_extension_command` | 実装済み |

### 動作確認済み一覧

| LSP | 動作確認状態 | 備考 |
| --- | --- | --- |
| abaplint | 未確認 | ローカル環境で未接続 |
| AS2 Language Support | 未確認 | ローカル環境で未接続 |
| Titan Language Server (ASN.1) | 未確認 | ローカル環境で未接続 |
| ada_language_server | 未確認 | ローカル環境で未接続 |
| agda-language-server | 未確認 | ローカル環境で未接続 |
| aml-language-server (AML / AsyncAPI / OpenAPI / RAML) | 未確認 | ローカル環境で未接続 |
| ansible-language-server | 未確認 | ローカル環境で未接続 |
| Angular Language Server (ngserver) | 未確認 | ローカル環境で未接続 |
| AntlrVSIX | 未確認 | ローカル環境で未接続 |
| vscode-apielements | 未確認 | ローカル環境で未接続 |
| apl-language-server | 未確認 | ローカル環境で未接続 |
| camel-language-server | 未確認 | ローカル環境で未接続 |
| apache-dispatcher-config-language-server | 未確認 | ローカル環境で未接続 |
| apex-jorje-lsp | 未確認 | ローカル環境で未接続 |
| astro-ls | 未確認 | ローカル環境で未接続 |
| awk-language-server | 未確認 | ローカル環境で未接続 |
| docker-language-server (Bake / Compose / Dockerfiles) | 未確認 | ローカル環境で未接続 |
| ballerina-language-server | 未確認 | ローカル環境で未接続 |
| bash-language-server | 未確認 | ローカル環境で未接続 |
| rech-editor-batch | 未確認 | ローカル環境で未接続 |
| bazel-lsp | 未確認 | ローカル環境で未接続 |
| bicep-language-server | 未確認 | ローカル環境で未接続 |
| bitbake-language-server | 未確認 | ローカル環境で未接続 |
| bsl-language-server | 未確認 | ローカル環境で未接続 |
| boriel-basic-lsp | 未確認 | ローカル環境で未接続 |
| brighterscript-language-server | 未確認 | ローカル環境で未接続 |
| b-language-server (B/ProB) | 未確認 | ローカル環境で未接続 |
| caddyfile-language-server | 未確認 | ローカル環境で未接続 |
| cds-lsp | 未確認 | ローカル環境で未接続 |
| vscode-css-language-server | 未確認 | ローカル環境で未接続 |
| ceylon-language-server | 未確認 | ローカル環境で未接続 |
| clarity-lsp | 未確認 | ローカル環境で未接続 |
| clojure-lsp | 未確認 | ローカル環境で未接続 |
| cmake-language-server / neocmakelsp | 未確認 | ローカル環境で未接続 |
| cl-lsp | 未確認 | ローカル環境で未接続 |
| chapel-language-server | 未確認 | ローカル環境で未接続 |
| coq-lsp / vscoq-language-server | 未確認 | ローカル環境で未接続 |
| rech-editor-cobol / cobol-language-server | 未確認 | ローカル環境で未接続 |
| codeql-language-server | 未確認 | ローカル環境で未接続 |
| coffeesense | 未確認 | ローカル環境で未接続 |
| crystalline / scry | 未確認 | ローカル環境で未接続 |
| cwl-language-server / benten | 未確認 | ローカル環境で未接続 |
| cucumber-language-server | 未確認 | ローカル環境で未接続 |
| cyright-langserver | 未確認 | ローカル環境で未接続 |
| serve-d / dls | 未確認 | ローカル環境で未接続 |
| dart language server | 未確認 | ローカル環境で未接続 |
| datapack-language-server | 未確認 | ローカル環境で未接続 |
| debputy lsp server | 未確認 | ローカル環境で未接続 |
| DelphiLSP | 未確認 | ローカル環境で未接続 |
| denizen-language-server | 未確認 | ローカル環境で未接続 |
| dts-lsp | 未確認 | ローカル環境で未接続 |
| denols | 未確認 | ローカル環境で未接続 |
| dockerfile-language-server | 未確認 | ローカル環境で未接続 |
| dm-langserver | 未確認 | ローカル環境で未接続 |
| egglog-language-server | 未確認 | ローカル環境で未接続 |
| ellsp | 未確認 | ローカル環境で未接続 |
| sourcer / erlang_ls / ELP | 未確認 | ローカル環境で未接続 |
| els | 未確認 | ローカル環境で未接続 |
| elixir-ls | 未確認 | ローカル環境で未接続 |
| elm-language-server | 未確認 | ローカル環境で未接続 |
| ember-language-server | 未確認 | ローカル環境で未接続 |
| fsharp-language-server / fsautocomplete | 未確認 | ローカル環境で未接続 |
| fish-lsp | 未確認 | ローカル環境で未接続 |
| fluent-bit-lsp | 未確認 | ローカル環境で未接続 |
| fortran-language-server / fortls | 未確認 | ローカル環境で未接続 |
| fuzion-lsp | 未確認 | ローカル環境で未接続 |
| glsl-language-server | 未確認 | ローカル環境で未接続 |
| mcshader-lsp | 未確認 | ローカル環境で未接続 |
| gauge-language-server | 未確認 | ローカル環境で未接続 |
| godot (GDScript LSP) | 未確認 | ローカル環境で未接続 |
| gleam lsp | 未確認 | ローカル環境で未接続 |
| glint-language-server | 未確認 | ローカル環境で未接続 |
| gluon-language-server | 未確認 | ローカル環境で未接続 |
| gn-language-server | 未確認 | ローカル環境で未接続 |
| go-langserver (sourcegraph-go) | 未確認 | ローカル環境で未接続 |
| graphql-language-service-server / gql-language-server | 未確認 | ローカル環境で未接続 |
| dot-language-server | 未確認 | ローカル環境で未接続 |
| grain-language-server | 未確認 | ローカル環境で未接続 |
| groovy-language-server / npm-groovy-lint-language-server | 未確認 | ローカル環境で未接続 |
| vscode-html-languageserver / superhtml | 未確認 | ローカル環境で未接続 |
| haskell-language-server | 未確認 | ローカル環境で未接続 |
| haxe-language-server | 未確認 | ローカル環境で未接続 |
| helm-ls | 未確認 | ローカル環境で未接続 |
| HlslTools.LanguageServer | 未確認 | ローカル環境で未接続 |
| ink-language-server | 未確認 | ローカル環境で未接続 |
| isabelle vscode_server | 未確認 | ローカル環境で未接続 |
| idris2-lsp | 未確認 | ローカル環境で未接続 |
| jdtls / java-language-server | 未確認 | ローカル環境で未接続 |
| quick-lint-js | 未確認 | ローカル環境で未接続 |
| flow / flow-language-server | 未確認 | ローカル環境で未接続 |
| javascript-typescript-langserver / biome_lsp | 未確認 | ローカル環境で未接続 |
| jcl-language-server | 未確認 | ローカル環境で未接続 |
| jimmer-dto-lsp | 未確認 | ローカル環境で未接続 |
| vscode-json-languageserver | 未確認 | ローカル環境で未接続 |
| jsonnet-language-server | 未確認 | ローカル環境で未接続 |
| Julia language server | 未確認 | ローカル環境で未接続 |
| kconfig-language-server | 未確認 | ローカル環境で未接続 |
| vscode-kdl | 未確認 | ローカル環境で未接続 |
| kedro-language-server | 未確認 | ローカル環境で未接続 |
| kos-language-server | 未確認 | ローカル環境で未接続 |
| kerml-language-server | 未確認 | ローカル環境で未接続 |
| kotlin-language-server / kotlin-lsp | 未確認 | ローカル環境で未接続 |
| typecobol-language-server-robot | 未確認 | ローカル環境で未接続 |
| languagetool-languageserver / ltex-ls | 未確認 | ローカル環境で未接続 |
| lark-parser-language-server | 未確認 | ローカル環境で未接続 |
| texlab | 未確認 | ローカル環境で未接続 |
| Lean4 | 未確認 | ローカル環境で未接続 |
| loxcraft | 未確認 | ローカル環境で未接続 |
| lpc-language-server | 未確認 | ローカル環境で未接続 |
| lua-lsp / lua-language-server / LuaHelper | 未確認 | ローカル環境で未接続 |
| theme-check-language-server | 未確認 | ローカル環境で未接続 |
| LPG-language-server | 未確認 | ローカル環境で未接続 |
| make-lsp-vscode | 未確認 | ローカル環境で未接続 |
| marksman / markmark / vscode-markdown-languageserver | 未確認 | ローカル環境で未接続 |
| matlab-language-server | 未確認 | ローカル環境で未接続 |
| mdx-analyzer | 未確認 | ローカル環境で未接続 |
| m68k-lsp | 未確認 | ローカル環境で未接続 |
| msbuild-language-server | 未確認 | ローカル環境で未接続 |
| asm-lsp | 未確認 | ローカル環境で未接続 |
| nginx-language-server | 未確認 | ローカル環境で未接続 |
| nimlsp | 未確認 | ローカル環境で未接続 |
| nobl9-vscode | 未確認 | ローカル環境で未接続 |
| ocamllsp | 未確認 | ローカル環境で未接続 |
| ols | 未確認 | ローカル環境で未接続 |
| abl-language-server | 未確認 | ローカル環境で未接続 |
| ov-language-server | 未確認 | ローカル環境で未接続 |
| papyrus-lang | 未確認 | ローカル環境で未接続 |
| aws-lsp-partiql | 未確認 | ローカル環境で未接続 |
| perl-languageserver / pls / perlnavigator | 未確認 | ローカル環境で未接続 |
| pest-ide-tools | 未確認 | ローカル環境で未接続 |
| crane / intelephense / php-language-server / serenata / phan / phpactor | 未確認 | ローカル環境で未接続 |
| phpunit-language-server | 未確認 | ローカル環境で未接続 |
| zopeneditor (PL/I) | 未確認 | ローカル環境で未接続 |
| plsql-language-server | 未確認 | ローカル環境で未接続 |
| polymer-editor-service | 未確認 | ローカル環境で未接続 |
| powerpc-support | 未確認 | ローカル環境で未接続 |
| powershell-editor-services | 未確認 | ローカル環境で未接続 |
| promql-langserver | 未確認 | ローカル環境で未接続 |
| protols / protobuf-language-server / buf lsp | 未確認 | ローカル環境で未接続 |
| purescript-language-server | 未確認 | ローカル環境で未接続 |
| puppet-languageserver | 未確認 | ローカル環境で未接続 |
| ty / pydev / pyright-langserver / pyrefly / basedpyright / pylsp / jedi-language-server / pylyzer / zuban | 未確認 | ローカル環境で未接続 |
| ponyls | 未確認 | ローカル環境で未接続 |
| qsharp-language-server | 未確認 | ローカル環境で未接続 |
| ts_query_ls | 未確認 | ローカル環境で未接続 |
| languageserver (R) | 未確認 | ローカル環境で未接続 |
| racket-langserver | 未確認 | ローカル環境で未接続 |
| rainlanguageserver | 未確認 | ローカル環境で未接続 |
| raku-navigator | 未確認 | ローカル環境で未接続 |
| raml-language-server | 未確認 | ローカル環境で未接続 |
| rascal-language-server | 未確認 | ローカル環境で未接続 |
| reason-language-server | 未確認 | ローカル環境で未接続 |
| redlangserver | 未確認 | ローカル環境で未接続 |
| regal-language-server | 未確認 | ローカル環境で未接続 |
| rel-ls | 未確認 | ローカル環境で未接続 |
| rescript-language-server | 未確認 | ローカル環境で未接続 |
| zopeneditor (REXX) | 未確認 | ローカル環境で未接続 |
| robotcode / robotframework-lsp | 未確認 | ローカル環境で未接続 |
| robots-txt-language-server | 未確認 | ローカル環境で未接続 |
| solargraph / language_server-ruby / sorbet / orbacle / ruby_language_server / ruby-lsp | 未確認 | ローカル環境で未接続 |
| rust-analyzer | 未確認 | ローカル環境で未接続 |
| metals / dragos-vscode-scala | 未確認 | ローカル環境で未接続 |
| scheme-langserver | 未確認 | ローカル環境で未接続 |
| shader-language-server | 未確認 | ローカル環境で未接続 |
| slint-lsp | 未確認 | ローカル環境で未接続 |
| pharolanguageserver | 未確認 | ローカル環境で未接続 |
| smithy-language-server | 未確認 | ローカル環境で未接続 |
| snyk-ls | 未確認 | ローカル環境で未接続 |
| qlue-ls / sparql-language-server | 未確認 | ローカル環境で未接続 |
| esbonio | 未確認 | ローカル環境で未接続 |
| sqls | 未確認 | ローカル環境で未接続 |
| millet | 未確認 | ローカル環境で未接続 |
| stimulus-language-server | 未確認 | ローカル環境で未接続 |
| stylable-language-server | 未確認 | ローカル環境で未接続 |
| svelteserver | 未確認 | ローカル環境で未接続 |
| sway-lsp | 未確認 | ローカル環境で未接続 |
| sourcekit-lsp | 未確認 | ローカル環境で未接続 |
| sysml2-language-server | 未確認 | ローカル環境で未接続 |
| sysl-language-server | 未確認 | ローカル環境で未接続 |
| systemd-language-server | 未確認 | ローカル環境で未接続 |
| systemtap-language-server | 未確認 | ローカル環境で未接続 |
| svls / sigasi / verible-verilog-ls / slang-server | 未確認 | ローカル環境で未接続 |
| sqltoolsservice (T-SQL) | 未確認 | ローカル環境で未接続 |
| tads3-language-server | 未確認 | ローカル環境で未接続 |
| teal-language-server | 未確認 | ローカル環境で未接続 |
| terraform-lsp / terraform-ls | 未確認 | ローカル環境で未接続 |
| thrift-ls (software-mansion / ocfbnj) | 未確認 | ローカル環境で未接続 |
| tibbo-basic-ls | 未確認 | ローカル環境で未接続 |
| taplo / tombi | 未確認 | ローカル環境で未接続 |
| trinols | 未確認 | ローカル環境で未接続 |
| ntt / titan-language-server (TTCN-3) | 未確認 | ローカル環境で未接続 |
| turtle-language-server | 未確認 | ローカル環境で未接続 |
| tailwindcss-language-server | 未確認 | ローカル環境で未接続 |
| twig-language-server | 未確認 | ローカル環境で未接続 |
| typecobol-language-server | 未確認 | ローカル環境で未接続 |
| typescript-language-server | 未確認 | ローカル環境で未接続 |
| tinymist / typst-lsp | 未確認 | ローカル環境で未接続 |
| v-analyzer | 未確認 | ローカル環境で未接続 |
| vala-language-server | 未確認 | ローカル環境で未接続 |
| vdmj-lsp | 未確認 | ローカル環境で未接続 |
| veryl-ls | 未確認 | ローカル環境で未接続 |
| vhdl_ls / sigasi / vhdl for professionals | 未確認 | ローカル環境で未接続 |
| vim-language-server | 未確認 | ローカル環境で未接続 |
| visualforce-language-server | 未確認 | ローカル環境で未接続 |
| vls / vue-language-server | 未確認 | ローカル環境で未接続 |
| wasm-language-tools / wasm-language-server | 未確認 | ローカル環境で未接続 |
| wgsl-analyzer | 未確認 | ローカル環境で未接続 |
| wikitext-language-server | 未確認 | ローカル環境で未接続 |
| wing-language-server | 未確認 | ローカル環境で未接続 |
| lsp-wl / LSPServer / wlsp | 未確認 | ローカル環境で未接続 |
| wxml-languageserver | 未確認 | ローカル環境で未接続 |
| xml-language-server / lemminx | 未確認 | ローカル環境で未接続 |
| miniyaml-language-server | 未確認 | ローカル環境で未接続 |
| vscode-yaml-languageservice / yaml-language-server | 未確認 | ローカル環境で未接続 |
| yara-language-server | 未確認 | ローカル環境で未接続 |
| yang-lsp | 未確認 | ローカル環境で未接続 |
| zls | 未確認 | ローカル環境で未接続 |
| nil / nixd | 未確認 | ローカル環境で未接続 |
| efm-langserver | 未確認 | ローカル環境で未接続 |
| diagnostic-languageserver | 未確認 | ローカル環境で未接続 |
| tagls | 未確認 | ローカル環境で未接続 |
| sonarlint-language-server | 未確認 | ローカル環境で未接続 |
| testing-language-server | 未確認 | ローカル環境で未接続 |
| copilot-language-server | 未確認 | ローカル環境で未接続 |
| harper | 未確認 | ローカル環境で未接続 |
| gopls | 確認済み | ローカル環境で実行確認済み |
| hlasm-language-server | 未確認 | ローカル環境で未接続 |
| ibmi-languages | 未確認 | ローカル環境で未接続 |
| clangd | 未確認 | ローカルに `clangd` 未インストールのため |
| ccls | 未確認 | ローカルに `ccls` 未インストールのため |
| csharp (OmniSharp / LanguageServer.NET / csharp-ls系) | 未確認 | ローカルに OmniSharp/LanguageServer.NET/csharp-ls 系LSP 未インストールのため |
| cquery | 未確認 | ローカルに `cquery` 未インストールのため |
| cpptools | 未確認 | ローカルに `cpptools` / `cpptools-srv` 未インストールのため |
| qmlls | 未確認 | ローカルに `qmlls` 未インストールのため |
