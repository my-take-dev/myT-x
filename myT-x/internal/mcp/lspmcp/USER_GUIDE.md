# generic-lsp-mcp ユーザーガイド

このドキュメントは、`generic-lsp-mcp` を「使う人」向けです。

## 1. これは何か

`generic-lsp-mcp` は、任意のLSPサーバー（`gopls`、`rust-analyzer` など）をMCP経由で使えるようにするサーバーです。

## 2. 実行モデル

常駐型です。

1. MCPクライアントが `generic-lsp-mcp` を起動
2. `generic-lsp-mcp` がLSPサーバーを起動
3. 以後のツール呼び出しは同じプロセスに送信

ツール呼び出しごとに再起動する方式ではありません。

## 3. 必要なもの（実行時）

1. `generic-lsp-mcp` 実行ファイル
2. 利用したいLSPサーバー
3. LSPサーバーコマンドがPATHで実行できること

注: 既に `.exe` がある場合、Go本体は不要です。

## 4. MCPクライアントへの登録

クライアント設定例（一般形）:

```json
{
  "mcpServers": {
    "generic-lsp": {
      "command": "C:\\\\path\\\\to\\\\generic-lspmcp.exe",
      "args": [
        "-lsp", "gopls",
        "-root", "C:\\\\path\\\\to\\\\project"
      ]
    }
  }
}
```

## 5. オプション

- `-lsp` (必須): LSPサーバー実行ファイル
- `-lsp-arg`: LSPサーバー引数（複数回指定可）
- `-root`: ワークスペースルート
- `-init-options`: `initialize.initializationOptions` のJSON文字列
- `-gopls-pull-diagnostics`: `gopls` 接続時のみ `initializationOptions.pullDiagnostics=true` を付与
- `-request-timeout-ms`: LSPリクエストタイムアウト
- `-open-delay-ms`: `didOpen` / `didChange` 後の待機ms
- `-language-id`: 拡張子推定不可時のフォールバック
- `-log-file`: ログ出力先

## 6. 提供ツール

基本ツール（常時有効）:

注: 基本ツールは言語共通です。実際に使える機能は接続中LSPのcapabilityに依存し、`-lsp`/`-lsp-arg` から判定した `lsp_pkg/*` の言語拡張（例: Go(gopls), C/C++(clangd/ccls/cquery/cpptools), C#, QML）と併せて判断します。

- `lsp_check_capabilities`
- `lsp_get_hover`
- `lsp_get_definitions`
- `lsp_get_declarations`
- `lsp_get_type_definitions`
- `lsp_get_implementations`
- `lsp_find_references`
- `lsp_get_document_symbols`
- `lsp_get_workspace_symbols`
- `lsp_resolve_workspace_symbol`
- `lsp_get_completion`
- `lsp_resolve_completion_item`
- `lsp_get_signature_help`
- `lsp_get_diagnostics`
- `lsp_get_workspace_diagnostics`
- `lsp_get_code_actions`
- `lsp_resolve_code_action`
- `lsp_format_document`
- `lsp_format_range`
- `lsp_format_on_type`
- `lsp_rename_symbol`
- `lsp_prepare_rename`
- `lsp_execute_command`

LSP固有拡張ツール（`-lsp`/`-lsp-arg` の判定で追加）:

- `abap_list_extension_commands`（ABAP / abaplint時）
- `abap_execute_extension_command`（ABAP / abaplint時）
- `as2_list_extension_commands`（ActionScript 2.0 / AS2 Language Support時）
- `as2_execute_extension_command`（ActionScript 2.0 / AS2 Language Support時）
- `asn1_list_extension_commands`（ASN.1 / Titan Language Server時）
- `asn1_execute_extension_command`（ASN.1 / Titan Language Server時）
- `ada_list_extension_commands`（Ada/SPARK / ada_language_server時）
- `ada_execute_extension_command`（Ada/SPARK / ada_language_server時）
- `agda_list_extension_commands`（Agda / agda-language-server時）
- `agda_execute_extension_command`（Agda / agda-language-server時）
- `aml_list_extension_commands`（AML / AsyncAPI / OpenAPI / RAML / AML Language Server時）
- `aml_execute_extension_command`（AML / AsyncAPI / OpenAPI / RAML / AML Language Server時）
- `ansible_list_extension_commands`（Ansible / ansible-language-server時）
- `ansible_execute_extension_command`（Ansible / ansible-language-server時）
- `angular_list_extension_commands`（Angular / Angular Language Server時）
- `angular_execute_extension_command`（Angular / Angular Language Server時）
- `antlr_list_extension_commands`（Antlr / AntlrVSIX時）
- `antlr_execute_extension_command`（Antlr / AntlrVSIX時）
- `apielements_list_extension_commands`（API Elements / vscode-apielements時）
- `apielements_execute_extension_command`（API Elements / vscode-apielements時）
- `apl_list_extension_commands`（APL / APL Language Server時）
- `apl_execute_extension_command`（APL / APL Language Server時）
- `camel_list_extension_commands`（Apache Camel / Apache Camel Language Server時）
- `camel_execute_extension_command`（Apache Camel / Apache Camel Language Server時）
- `apachedispatcher_list_extension_commands`（Apache Dispatcher Config / vscode-apache-dispatcher-config-language-support時）
- `apachedispatcher_execute_extension_command`（Apache Dispatcher Config / vscode-apache-dispatcher-config-language-support時）
- `apex_list_extension_commands`（Apex / Salesforce VS Code Apex extension時）
- `apex_execute_extension_command`（Apex / Salesforce VS Code Apex extension時）
- `astro_list_extension_commands`（Astro / withastro/language-tools時）
- `astro_execute_extension_command`（Astro / withastro/language-tools時）
- `awk_list_extension_commands`（AWK / AWK Language Server時）
- `awk_execute_extension_command`（AWK / AWK Language Server時）
- `bake_list_extension_commands`（Bake / docker-language-server時）
- `bake_execute_extension_command`（Bake / docker-language-server時）
- `ballerina_list_extension_commands`（Ballerina / Ballerina Language Server時）
- `ballerina_execute_extension_command`（Ballerina / Ballerina Language Server時）
- `bash_list_extension_commands`（Bash / bash-language-server時）
- `bash_execute_extension_command`（Bash / bash-language-server時）
- `batch_list_extension_commands`（Batch / rech-editor-batch時）
- `batch_execute_extension_command`（Batch / rech-editor-batch時）
- `bazel_list_extension_commands`（Bazel / bazel-lsp時）
- `bazel_execute_extension_command`（Bazel / bazel-lsp時）
- `bicep_list_extension_commands`（Bicep / Bicep時）
- `bicep_execute_extension_command`（Bicep / Bicep時）
- `bitbake_list_extension_commands`（BitBake / BitBake Language Server時）
- `bitbake_execute_extension_command`（BitBake / BitBake Language Server時）
- `bsl_list_extension_commands`（1C Enterprise / BSL Language Server時）
- `bsl_execute_extension_command`（1C Enterprise / BSL Language Server時）
- `boriel_list_extension_commands`（Boriel Basic / boriel-basic-lsp時）
- `boriel_execute_extension_command`（Boriel Basic / boriel-basic-lsp時）
- `brighterscript_list_extension_commands`（BrightScript/BrighterScript / brighterscript時）
- `brighterscript_execute_extension_command`（BrightScript/BrighterScript / brighterscript時）
- `bprob_list_extension_commands`（B/ProB / b-language-server時）
- `bprob_execute_extension_command`（B/ProB / b-language-server時）
- `caddy_list_extension_commands`（caddy / caddyfile-language-server時）
- `caddy_execute_extension_command`（caddy / caddyfile-language-server時）
- `cds_list_extension_commands`（CDS / @sap/cds-lsp時）
- `cds_execute_extension_command`（CDS / @sap/cds-lsp時）
- `cssls_list_extension_commands`（CSS/LESS/SASS / vscode-css-languageserver時）
- `cssls_execute_extension_command`（CSS/LESS/SASS / vscode-css-languageserver時）
- `ceylon_list_extension_commands`（Ceylon / vscode-ceylon時）
- `ceylon_execute_extension_command`（Ceylon / vscode-ceylon時）
- `clarity_list_extension_commands`（Clarity / clarity-lsp時）
- `clarity_execute_extension_command`（Clarity / clarity-lsp時）
- `clojure_list_extension_commands`（Clojure / clojure-lsp時）
- `clojure_execute_extension_command`（Clojure / clojure-lsp時）
- `cmake_list_extension_commands`（CMake / cmake-language-server, neocmakelsp時）
- `cmake_execute_extension_command`（CMake / cmake-language-server, neocmakelsp時）
- `commonlisp_list_extension_commands`（Common Lisp / cl-lsp時）
- `commonlisp_execute_extension_command`（Common Lisp / cl-lsp時）
- `chapel_list_extension_commands`（Chapel / chapel-language-server時）
- `chapel_execute_extension_command`（Chapel / chapel-language-server時）
- `coq_list_extension_commands`（Coq / coq-lsp, vscoq時）
- `coq_execute_extension_command`（Coq / coq-lsp, vscoq時）
- `cobol_list_extension_commands`（COBOL / rech-editor-cobol, cobol-language-support時）
- `cobol_execute_extension_command`（COBOL / rech-editor-cobol, cobol-language-support時）
- `codeql_list_extension_commands`（CodeQL / codeql-language-server時）
- `codeql_execute_extension_command`（CodeQL / codeql-language-server時）
- `coffeescript_list_extension_commands`（CoffeeScript / coffeesense時）
- `coffeescript_execute_extension_command`（CoffeeScript / coffeesense時）
- `crystal_list_extension_commands`（Crystal / crystalline, scry時）
- `crystal_execute_extension_command`（Crystal / crystalline, scry時）
- `cwl_list_extension_commands`（CWL / benten時）
- `cwl_execute_extension_command`（CWL / benten時）
- `cucumber_list_extension_commands`（Cucumber/Gherkin / Cucumber Language Server時）
- `cucumber_execute_extension_command`（Cucumber/Gherkin / Cucumber Language Server時）
- `cython_list_extension_commands`（Cython / cyright時）
- `cython_execute_extension_command`（Cython / cyright時）
- `dlang_list_extension_commands`（D / serve-d, dls時）
- `dlang_execute_extension_command`（D / serve-d, dls時）
- `dart_list_extension_commands`（Dart / Dart SDK時）
- `dart_execute_extension_command`（Dart / Dart SDK時）
- `datapack_list_extension_commands`（Data Pack / Data-pack Language Server時）
- `datapack_execute_extension_command`（Data Pack / Data-pack Language Server時）
- `debian_list_extension_commands`（Debian Packaging files / debputy lsp server時）
- `debian_execute_extension_command`（Debian Packaging files / debputy lsp server時）
- `delphi_list_extension_commands`（Delphi / DelphiLSP時）
- `delphi_execute_extension_command`（Delphi / DelphiLSP時）
- `denizenscript_list_extension_commands`（DenizenScript / DenizenVSCode時）
- `denizenscript_execute_extension_command`（DenizenScript / DenizenVSCode時）
- `devicetree_list_extension_commands`（devicetree / dts-lsp時）
- `devicetree_execute_extension_command`（devicetree / dts-lsp時）
- `deno_list_extension_commands`（Deno / deno lsp時）
- `deno_execute_extension_command`（Deno / deno lsp時）
- `dockerfile_list_extension_commands`（Dockerfiles / dockerfile-language-server時）
- `dockerfile_execute_extension_command`（Dockerfiles / dockerfile-language-server時）
- `dreammaker_list_extension_commands`（DreamMaker / DreamMaker Language Server時）
- `dreammaker_execute_extension_command`（DreamMaker / DreamMaker Language Server時）
- `egglog_list_extension_commands`（Egglog / egglog-language-server時）
- `egglog_execute_extension_command`（Egglog / egglog-language-server時）
- `emacslisp_list_extension_commands`（Emacs Lisp / ellsp時）
- `emacslisp_execute_extension_command`（Emacs Lisp / ellsp時）
- `erlang_list_extension_commands`（Erlang / sourcer, erlang_ls, ELP時）
- `erlang_execute_extension_command`（Erlang / sourcer, erlang_ls, ELP時）
- `erg_list_extension_commands`（Erg / els時）
- `erg_execute_extension_command`（Erg / els時）
- `elixir_list_extension_commands`（Elixir / elixir-ls時）
- `elixir_execute_extension_command`（Elixir / elixir-ls時）
- `elm_list_extension_commands`（Elm / elm-language-server時）
- `elm_execute_extension_command`（Elm / elm-language-server時）
- `ember_list_extension_commands`（Ember / Ember Language Server時）
- `ember_execute_extension_command`（Ember / Ember Language Server時）
- `fsharp_list_extension_commands`（F# / fsharp-language-server, FsAutoComplete時）
- `fsharp_execute_extension_command`（F# / fsharp-language-server, FsAutoComplete時）
- `fish_list_extension_commands`（fish / fish-lsp時）
- `fish_execute_extension_command`（fish / fish-lsp時）
- `fluentbit_list_extension_commands`（fluent-bit / fluent-bit-lsp時）
- `fluentbit_execute_extension_command`（fluent-bit / fluent-bit-lsp時）
- `fortran_list_extension_commands`（Fortran / fortran-language-server, fortls時）
- `fortran_execute_extension_command`（Fortran / fortran-language-server, fortls時）
- `fuzion_list_extension_commands`（Fuzion / Fuzion Language Server時）
- `fuzion_execute_extension_command`（Fuzion / Fuzion Language Server時）
- `glsl_list_extension_commands`（GLSL / glsl-language-server時）
- `glsl_execute_extension_command`（GLSL / glsl-language-server時）
- `mcshader_list_extension_commands`（GLSL for Minecraft / mcshader-lsp時）
- `mcshader_execute_extension_command`（GLSL for Minecraft / mcshader-lsp時）
- `gauge_list_extension_commands`（Gauge / Gauge Language Server時）
- `gauge_execute_extension_command`（Gauge / Gauge Language Server時）
- `gdscript_list_extension_commands`（GDScript / Godot時）
- `gdscript_execute_extension_command`（GDScript / Godot時）
- `gleam_list_extension_commands`（Gleam / gleam時）
- `gleam_execute_extension_command`（Gleam / gleam時）
- `glimmer_list_extension_commands`（Glimmer templates / Glint時）
- `glimmer_execute_extension_command`（Glimmer templates / Glint時）
- `gluon_list_extension_commands`（Gluon / Gluon Language Server時）
- `gluon_execute_extension_command`（Gluon / Gluon Language Server時）
- `gn_list_extension_commands`（GN / gn-language-server時）
- `gn_execute_extension_command`（GN / gn-language-server時）
- `sourcegraphgo_list_extension_commands`（Go / sourcegraph-go時）
- `sourcegraphgo_execute_extension_command`（Go / sourcegraph-go時）
- `graphql_list_extension_commands`（GraphQL / Official GraphQL Language Server, GQL Language Server時）
- `graphql_execute_extension_command`（GraphQL / Official GraphQL Language Server, GQL Language Server時）
- `dot_list_extension_commands`（Graphviz/DOT / dot-language-server時）
- `dot_execute_extension_command`（Graphviz/DOT / dot-language-server時）
- `grain_list_extension_commands`（Grain / grain時）
- `grain_execute_extension_command`（Grain / grain時）
- `groovy_list_extension_commands`（Groovy / groovy-language-server, Groovy Language Server, VsCode Groovy Lint Language Server時）
- `groovy_execute_extension_command`（Groovy / groovy-language-server, Groovy Language Server, VsCode Groovy Lint Language Server時）
- `html_list_extension_commands`（HTML / vscode-html-languageserver, SuperHTML時）
- `html_execute_extension_command`（HTML / vscode-html-languageserver, SuperHTML時）
- `haskell_list_extension_commands`（Haskell / Haskell Language Server (HLS)時）
- `haskell_execute_extension_command`（Haskell / Haskell Language Server (HLS)時）
- `haxe_list_extension_commands`（Haxe / Haxe Language Server時）
- `haxe_execute_extension_command`（Haxe / Haxe Language Server時）
- `helm_list_extension_commands`（Helm (Kubernetes) / helm-ls時）
- `helm_execute_extension_command`（Helm (Kubernetes) / helm-ls時）
- `hlsl_list_extension_commands`（HLSL / HLSL Tools時）
- `hlsl_execute_extension_command`（HLSL / HLSL Tools時）
- `ink_list_extension_commands`（ink! / ink! Language Server時）
- `ink_execute_extension_command`（ink! / ink! Language Server時）
- `isabelle_list_extension_commands`（Isabelle / sources時）
- `isabelle_execute_extension_command`（Isabelle / sources時）
- `idris2_list_extension_commands`（Idris2 / idris2-lsp時）
- `idris2_execute_extension_command`（Idris2 / idris2-lsp時）
- `java_list_extension_commands`（Java / Eclipse JDT LS, Java Compiler API-based Java support時）
- `java_execute_extension_command`（Java / Eclipse JDT LS, Java Compiler API-based Java support時）
- `javascript_list_extension_commands`（JavaScript / quick-lint-js時）
- `javascript_execute_extension_command`（JavaScript / quick-lint-js時）
- `flow_list_extension_commands`（JavaScript Flow / flow, flow-language-server時）
- `flow_execute_extension_command`（JavaScript Flow / flow, flow-language-server時）
- `jstypescript_list_extension_commands`（JavaScript-Typescript / sourcegraph javascript-typescript, biome_lsp時）
- `jstypescript_execute_extension_command`（JavaScript-Typescript / sourcegraph javascript-typescript, biome_lsp時）
- `jcl_list_extension_commands`（JCL / IBM Z Open Editor時）
- `jcl_execute_extension_command`（JCL / IBM Z Open Editor時）
- `jimmerdto_list_extension_commands`（Jimmer DTO / jimmer-dto-lsp時）
- `jimmerdto_execute_extension_command`（Jimmer DTO / jimmer-dto-lsp時）
- `jsonls_list_extension_commands`（JSON / vscode-json-languageserver時）
- `jsonls_execute_extension_command`（JSON / vscode-json-languageserver時）
- `jsonnet_list_extension_commands`（Jsonnet / jsonnet-language-server時）
- `jsonnet_execute_extension_command`（Jsonnet / jsonnet-language-server時）
- `julia_list_extension_commands`（Julia / Julia language server時）
- `julia_execute_extension_command`（Julia / Julia language server時）
- `kconfig_list_extension_commands`（Kconfig / kconfig-language-server時）
- `kconfig_execute_extension_command`（Kconfig / kconfig-language-server時）
- `kdl_list_extension_commands`（KDL / vscode-kdl時）
- `kdl_execute_extension_command`（KDL / vscode-kdl時）
- `kedro_list_extension_commands`（Kedro / Kedro VSCode Language Server時）
- `kedro_execute_extension_command`（Kedro / Kedro VSCode Language Server時）
- `kerboscript_list_extension_commands`（Kerboscript (kOS) / kos-language-server時）
- `kerboscript_execute_extension_command`（Kerboscript (kOS) / kos-language-server時）
- `kerml_list_extension_commands`（KerML / SysML2 Tools時）
- `kerml_execute_extension_command`（KerML / SysML2 Tools時）
- `kotlin_list_extension_commands`（Kotlin / kotlin-language-server, kotlin-lsp時）
- `kotlin_execute_extension_command`（Kotlin / kotlin-language-server, kotlin-lsp時）
- `typecobolrobot_list_extension_commands`（Language Server Robot / TypeCobol Language Server Robot時）
- `typecobolrobot_execute_extension_command`（Language Server Robot / TypeCobol Language Server Robot時）
- `languagetool_list_extension_commands`（LanguageTool / languagetool, ltex-ls時）
- `languagetool_execute_extension_command`（LanguageTool / languagetool, ltex-ls時）
- `lark_list_extension_commands`（Lark / lark-parser-language-server時）
- `lark_execute_extension_command`（Lark / lark-parser-language-server時）
- `latex_list_extension_commands`（LaTeX / texlab時）
- `latex_execute_extension_command`（LaTeX / texlab時）
- `lean4_list_extension_commands`（Lean4 / Lean4時）
- `lean4_execute_extension_command`（Lean4 / Lean4時）
- `lox_list_extension_commands`（Lox / loxcraft時）
- `lox_execute_extension_command`（Lox / loxcraft時）
- `lpc_list_extension_commands`（LPC / lpc-language-server時）
- `lpc_execute_extension_command`（LPC / lpc-language-server時）
- `lua_list_extension_commands`（Lua / lua-lsp, lua-language-server, LuaHelper時）
- `lua_execute_extension_command`（Lua / lua-lsp, lua-language-server, LuaHelper時）
- `liquid_list_extension_commands`（Liquid / theme-check時）
- `liquid_execute_extension_command`（Liquid / theme-check時）
- `lpg_list_extension_commands`（IBM LALR Parser Generator / LPG-language-server時）
- `lpg_execute_extension_command`（IBM LALR Parser Generator / LPG-language-server時）
- `make_list_extension_commands`（Make / make-lsp-vscode時）
- `make_execute_extension_command`（Make / make-lsp-vscode時）
- `markdown_list_extension_commands`（Markdown / Marksman, Markmark, vscode-markdown-languageserver時）
- `markdown_execute_extension_command`（Markdown / Marksman, Markmark, vscode-markdown-languageserver時）
- `matlab_list_extension_commands`（MATLAB / MATLAB-language-server時）
- `matlab_execute_extension_command`（MATLAB / MATLAB-language-server時）
- `mdx_list_extension_commands`（MDX / mdx-js/mdx-analyzer時）
- `mdx_execute_extension_command`（MDX / mdx-js/mdx-analyzer時）
- `m68k_list_extension_commands`（Motorola 68000 Assembly / m68k-lsp時）
- `m68k_execute_extension_command`（Motorola 68000 Assembly / m68k-lsp時）
- `msbuild_list_extension_commands`（MSBuild / msbuild-project-tools-vscode時）
- `msbuild_execute_extension_command`（MSBuild / msbuild-project-tools-vscode時）
- `asmlsp_list_extension_commands`（NASM/GO/GAS Assembly / asm-lsp時）
- `asmlsp_execute_extension_command`（NASM/GO/GAS Assembly / asm-lsp時）
- `nginx_list_extension_commands`（Nginx / nginx-language-server時）
- `nginx_execute_extension_command`（Nginx / nginx-language-server時）
- `nim_list_extension_commands`（Nim / nimlsp時）
- `nim_execute_extension_command`（Nim / nimlsp時）
- `nobl9yaml_list_extension_commands`（Nobl9 YAML / nobl9-vscode時）
- `nobl9yaml_execute_extension_command`（Nobl9 YAML / nobl9-vscode時）
- `ocamlreason_list_extension_commands`（OCaml/Reason / ocamllsp時）
- `ocamlreason_execute_extension_command`（OCaml/Reason / ocamllsp時）
- `odin_list_extension_commands`（Odin / ols時）
- `odin_execute_extension_command`（Odin / ols時）
- `openedgeabl_list_extension_commands`（OpenEdge ABL / ABL Language Server時）
- `openedgeabl_execute_extension_command`（OpenEdge ABL / ABL Language Server時）
- `openvalidation_list_extension_commands`（openVALIDATION / ov-language-server時）
- `openvalidation_execute_extension_command`（openVALIDATION / ov-language-server時）
- `papyrus_list_extension_commands`（Papyrus / papyrus-lang時）
- `papyrus_execute_extension_command`（Papyrus / papyrus-lang時）
- `partiql_list_extension_commands`（PartiQL / aws-lsp-partiql時）
- `partiql_execute_extension_command`（PartiQL / aws-lsp-partiql時）
- `perl_list_extension_commands`（Perl / Perl::LanguageServer, PLS, Perl Navigator時）
- `perl_execute_extension_command`（Perl / Perl::LanguageServer, PLS, Perl Navigator時）
- `pest_list_extension_commands`（Pest / Pest IDE Tools時）
- `pest_execute_extension_command`（Pest / Pest IDE Tools時）
- `php_list_extension_commands`（PHP / Crane, intelephense, php-language-server, Serenata, Phan, phpactor時）
- `php_execute_extension_command`（PHP / Crane, intelephense, php-language-server, Serenata, Phan, phpactor時）
- `phpunit_list_extension_commands`（PHPUnit / phpunit-language-server時）
- `phpunit_execute_extension_command`（PHPUnit / phpunit-language-server時）
- `pli_list_extension_commands`（IBM Enterprise PL/I for z/OS / IBM Z Open Editor時）
- `pli_execute_extension_command`（IBM Enterprise PL/I for z/OS / IBM Z Open Editor時）
- `plsql_list_extension_commands`（PL/SQL / plsql-language-server時）
- `plsql_execute_extension_command`（PL/SQL / plsql-language-server時）
- `polymer_list_extension_commands`（Polymer / polymer-editor-service時）
- `polymer_execute_extension_command`（Polymer / polymer-editor-service時）
- `powerpc_list_extension_commands`（PowerPC Assembly / PowerPC Support時）
- `powerpc_execute_extension_command`（PowerPC Assembly / PowerPC Support時）
- `powershell_list_extension_commands`（PowerShell / PowerShell Editor Services時）
- `powershell_execute_extension_command`（PowerShell / PowerShell Editor Services時）
- `promql_list_extension_commands`（PromQL / promql-langserver時）
- `promql_execute_extension_command`（PromQL / promql-langserver時）
- `protobuf_list_extension_commands`（Protocol Buffers / protols, protobuf-language-server, Buf Language Server時）
- `protobuf_execute_extension_command`（Protocol Buffers / protols, protobuf-language-server, Buf Language Server時）
- `purescript_list_extension_commands`（PureScript / purescript-language-server時）
- `purescript_execute_extension_command`（PureScript / purescript-language-server時）
- `puppet_list_extension_commands`（Puppet / puppet-languageserver時）
- `puppet_execute_extension_command`（Puppet / puppet-languageserver時）
- `python_list_extension_commands`（Python / ty, PyDev, Pyright, Pyrefly, basedpyright, python-lsp-server, jedi-language-server, pylyzer, zuban時）
- `python_execute_extension_command`（Python / ty, PyDev, Pyright, Pyrefly, basedpyright, python-lsp-server, jedi-language-server, pylyzer, zuban時）
- `pony_list_extension_commands`（Pony / PonyLS時）
- `pony_execute_extension_command`（Pony / PonyLS時）
- `qsharp_list_extension_commands`（Q# / Q# Language Server時）
- `qsharp_execute_extension_command`（Q# / Q# Language Server時）
- `query_list_extension_commands`（Query / ts_query_ls時）
- `query_execute_extension_command`（Query / ts_query_ls時）
- `rlang_list_extension_commands`（R / R language server時）
- `rlang_execute_extension_command`（R / R language server時）
- `racket_list_extension_commands`（Racket / racket-langserver時）
- `racket_execute_extension_command`（Racket / racket-langserver時）
- `rain_list_extension_commands`（Rain / RainLanguageServer時）
- `rain_execute_extension_command`（Rain / RainLanguageServer時）
- `raku_list_extension_commands`（Raku / Raku Navigator時）
- `raku_execute_extension_command`（Raku / Raku Navigator時）
- `raml_list_extension_commands`（RAML / raml-language-server時）
- `raml_execute_extension_command`（RAML / raml-language-server時）
- `rascal_list_extension_commands`（Rascal / rascal-language-server時）
- `rascal_execute_extension_command`（Rascal / rascal-language-server時）
- `reasonml_list_extension_commands`（ReasonML / reason-language-server時）
- `reasonml_execute_extension_command`（ReasonML / reason-language-server時）
- `red_list_extension_commands`（Red / redlangserver時）
- `red_execute_extension_command`（Red / redlangserver時）
- `rego_list_extension_commands`（Rego / Regal時）
- `rego_execute_extension_command`（Rego / Regal時）
- `rel_list_extension_commands`（REL / rel-ls時）
- `rel_execute_extension_command`（REL / rel-ls時）
- `rescript_list_extension_commands`（ReScript / rescript-vscode時）
- `rescript_execute_extension_command`（ReScript / rescript-vscode時）
- `rexx_list_extension_commands`（IBM TSO/E REXX / IBM Z Open Editor時）
- `rexx_execute_extension_command`（IBM TSO/E REXX / IBM Z Open Editor時）
- `robotframework_list_extension_commands`（Robot Framework / RobotCode, robotframework-lsp時）
- `robotframework_execute_extension_command`（Robot Framework / RobotCode, robotframework-lsp時）
- `robotstxt_list_extension_commands`（Robots.txt / vscode-robots-dot-txt-support時）
- `robotstxt_execute_extension_command`（Robots.txt / vscode-robots-dot-txt-support時）
- `ruby_list_extension_commands`（Ruby / solargraph, language_server-ruby, sorbet, orbacle, ruby_language_server, ruby-lsp時）
- `ruby_execute_extension_command`（Ruby / solargraph, language_server-ruby, sorbet, orbacle, ruby_language_server, ruby-lsp時）
- `rust_list_extension_commands`（Rust / rust-analyzer時）
- `rust_execute_extension_command`（Rust / rust-analyzer時）
- `scala_list_extension_commands`（Scala / dragos-vscode-scala, Metals時）
- `scala_execute_extension_command`（Scala / dragos-vscode-scala, Metals時）
- `scheme_list_extension_commands`（Scheme / scheme-langserver時）
- `scheme_execute_extension_command`（Scheme / scheme-langserver時）
- `shader_list_extension_commands`（Shader / shader-language-server時）
- `shader_execute_extension_command`（Shader / shader-language-server時）
- `slint_list_extension_commands`（Slint / slint-lsp時）
- `slint_execute_extension_command`（Slint / slint-lsp時）
- `pharo_list_extension_commands`（Smalltalk/Pharo / Pharo Language Server時）
- `pharo_execute_extension_command`（Smalltalk/Pharo / Pharo Language Server時）
- `smithy_list_extension_commands`（Smithy / smithy-language-server時）
- `smithy_execute_extension_command`（Smithy / smithy-language-server時）
- `snyk_list_extension_commands`（Snyk / snyk-ls時）
- `snyk_execute_extension_command`（Snyk / snyk-ls時）
- `sparql_list_extension_commands`（SPARQL / Qlue-ls, SPARQL Language Server時）
- `sparql_execute_extension_command`（SPARQL / Qlue-ls, SPARQL Language Server時）
- `sphinx_list_extension_commands`（Sphinx / esbonio時）
- `sphinx_execute_extension_command`（Sphinx / esbonio時）
- `sql_list_extension_commands`（SQL / sqls時）
- `sql_execute_extension_command`（SQL / sqls時）
- `standardml_list_extension_commands`（Standard ML / Millet時）
- `standardml_execute_extension_command`（Standard ML / Millet時）
- `stimulus_list_extension_commands`（Stimulus / Stimulus LSP時）
- `stimulus_execute_extension_command`（Stimulus / Stimulus LSP時）
- `stylable_list_extension_commands`（Stylable / stylable/language-service時）
- `stylable_execute_extension_command`（Stylable / stylable/language-service時）
- `svelte_list_extension_commands`（Svelte / svelte-language-server時）
- `svelte_execute_extension_command`（Svelte / svelte-language-server時）
- `sway_list_extension_commands`（Sway / sway-lsp時）
- `sway_execute_extension_command`（Sway / sway-lsp時）
- `swift_list_extension_commands`（Swift / SourceKit-LSP時）
- `swift_execute_extension_command`（Swift / SourceKit-LSP時）
- `sysml2_list_extension_commands`（SysML v2 / SysML2 Tools時）
- `sysml2_execute_extension_command`（SysML v2 / SysML2 Tools時）
- `sysl_list_extension_commands`（Sysl / Sysl LSP時）
- `sysl_execute_extension_command`（Sysl / Sysl LSP時）
- `systemd_list_extension_commands`（systemd / systemd-language-server時）
- `systemd_execute_extension_command`（systemd / systemd-language-server時）
- `systemtap_list_extension_commands`（Systemtap / Systemtap LSP時）
- `systemtap_execute_extension_command`（Systemtap / Systemtap LSP時）
- `systemverilog_list_extension_commands`（SystemVerilog / svls, Sigasi, Verible, slang-server時）
- `systemverilog_execute_extension_command`（SystemVerilog / svls, Sigasi, Verible, slang-server時）
- `tsql_list_extension_commands`（T-SQL / MS VS Code SQL extension時）
- `tsql_execute_extension_command`（T-SQL / MS VS Code SQL extension時）
- `tads3_list_extension_commands`（Tads3 / tads3tools時）
- `tads3_execute_extension_command`（Tads3 / tads3tools時）
- `teal_list_extension_commands`（Teal / teal-language-server時）
- `teal_execute_extension_command`（Teal / teal-language-server時）
- `terraform_list_extension_commands`（Terraform / terraform-lsp, terraform-ls時）
- `terraform_execute_extension_command`（Terraform / terraform-lsp, terraform-ls時）
- `thrift_list_extension_commands`（Thrift / thrift-ls時）
- `thrift_execute_extension_command`（Thrift / thrift-ls時）
- `tibbobasic_list_extension_commands`（Tibbo Basic / tibbo-basic時）
- `tibbobasic_execute_extension_command`（Tibbo Basic / tibbo-basic時）
- `toml_list_extension_commands`（TOML / Taplo, Tombi時）
- `toml_execute_extension_command`（TOML / Taplo, Tombi時）
- `trinosql_list_extension_commands`（Trino SQL / trinols時）
- `trinosql_execute_extension_command`（Trino SQL / trinols時）
- `ttcn3_list_extension_commands`（TTCN-3 / ntt, Titan Language Server時）
- `ttcn3_execute_extension_command`（TTCN-3 / ntt, Titan Language Server時）
- `turtle_list_extension_commands`（Turtle / Turtle Language Server時）
- `turtle_execute_extension_command`（Turtle / Turtle Language Server時）
- `tailwindcss_list_extension_commands`（Tailwind CSS / Tailwind Intellisense時）
- `tailwindcss_execute_extension_command`（Tailwind CSS / Tailwind Intellisense時）
- `twig_list_extension_commands`（Twig / Twig Language Server時）
- `twig_execute_extension_command`（Twig / Twig Language Server時）
- `typecobol_list_extension_commands`（TypeCobol / TypeCobol language server時）
- `typecobol_execute_extension_command`（TypeCobol / TypeCobol language server時）
- `typescriptls_list_extension_commands`（TypeScript / typescript-language-server時）
- `typescriptls_execute_extension_command`（TypeScript / typescript-language-server時）
- `typst_list_extension_commands`（Typst / tinymist, typst-lsp時）
- `typst_execute_extension_command`（Typst / tinymist, typst-lsp時）
- `vlang_list_extension_commands`（V / v-analyzer時）
- `vlang_execute_extension_command`（V / v-analyzer時）
- `vala_list_extension_commands`（Vala / vala-language-server時）
- `vala_execute_extension_command`（Vala / vala-language-server時）
- `vdm_list_extension_commands`（VDM-SL, VDM++, VDM-RT / VDMJ-LSP時）
- `vdm_execute_extension_command`（VDM-SL, VDM++, VDM-RT / VDMJ-LSP時）
- `veryl_list_extension_commands`（Veryl / Veryl Language Server時）
- `veryl_execute_extension_command`（Veryl / Veryl Language Server時）
- `vhdl_list_extension_commands`（VHDL / vhdl_ls, Sigasi, VHDL for Professionals時）
- `vhdl_execute_extension_command`（VHDL / vhdl_ls, Sigasi, VHDL for Professionals時）
- `viml_list_extension_commands`（Viml / vim-language-server時）
- `viml_execute_extension_command`（Viml / vim-language-server時）
- `visualforce_list_extension_commands`（Visualforce / Salesforce VS Code Visualforce extension時）
- `visualforce_execute_extension_command`（Visualforce / Salesforce VS Code Visualforce extension時）
- `vue_list_extension_commands`（Vue / vuejs/vetur, vuejs/language-tools時）
- `vue_execute_extension_command`（Vue / vuejs/vetur, vuejs/language-tools時）
- `wasm_list_extension_commands`（WebAssembly / wasm-language-tools, wasm-language-server時）
- `wasm_execute_extension_command`（WebAssembly / wasm-language-tools, wasm-language-server時）
- `wgsl_list_extension_commands`（WebGPU Shading Language / wgsl-analyzer時）
- `wgsl_execute_extension_command`（WebGPU Shading Language / wgsl-analyzer時）
- `wikitext_list_extension_commands`（Wikitext / VSCode-WikiParser時）
- `wikitext_execute_extension_command`（Wikitext / VSCode-WikiParser時）
- `wing_list_extension_commands`（Wing / Wing時）
- `wing_execute_extension_command`（Wing / Wing時）
- `wolfram_list_extension_commands`（Wolfram Language / lsp-wl, LSPServer, wlsp時）
- `wolfram_execute_extension_command`（Wolfram Language / lsp-wl, LSPServer, wlsp時）
- `wxml_list_extension_commands`（WXML / wxml-languageserver時）
- `wxml_execute_extension_command`（WXML / wxml-languageserver時）
- `xml_list_extension_commands`（XML / XML Language Server, XML Language Server (LemMinX)時）
- `xml_execute_extension_command`（XML / XML Language Server, XML Language Server (LemMinX)時）
- `miniyaml_list_extension_commands`（MiniYAML / ORAIDE時）
- `miniyaml_execute_extension_command`（MiniYAML / ORAIDE時）
- `yaml_list_extension_commands`（YAML / vscode-yaml-languageservice, yaml-language-server時）
- `yaml_execute_extension_command`（YAML / vscode-yaml-languageservice, yaml-language-server時）
- `yara_list_extension_commands`（YARA / YARA Language Server時）
- `yara_execute_extension_command`（YARA / YARA Language Server時）
- `yang_list_extension_commands`（YANG / yang-lsp時）
- `yang_execute_extension_command`（YANG / yang-lsp時）
- `zig_list_extension_commands`（Zig / zls時）
- `zig_execute_extension_command`（Zig / zls時）
- `nix_list_extension_commands`（Nix / nil, nixd時）
- `nix_execute_extension_command`（Nix / nil, nixd時）
- `efm_list_extension_commands`（* / efm-langserver時）
- `efm_execute_extension_command`（* / efm-langserver時）
- `diagnosticls_list_extension_commands`（* / diagnostic-languageserver時）
- `diagnosticls_execute_extension_command`（* / diagnostic-languageserver時）
- `tagls_list_extension_commands`（* / tagls時）
- `tagls_execute_extension_command`（* / tagls時）
- `sonarlint_list_extension_commands`（* / SonarLint Language Server時）
- `sonarlint_execute_extension_command`（* / SonarLint Language Server時）
- `testingls_list_extension_commands`（* / testing-language-server時）
- `testingls_execute_extension_command`（* / testing-language-server時）
- `copilot_list_extension_commands`（* / @github/copilot-language-server時）
- `copilot_execute_extension_command`（* / @github/copilot-language-server時）
- `harper_list_extension_commands`（* / harper時）
- `harper_execute_extension_command`（* / harper時）
- `hlasm_list_extension_commands`（IBM High Level Assembler / hlasm-language-server時）
- `hlasm_execute_extension_command`（IBM High Level Assembler / hlasm-language-server時）
- `ibmi_list_extension_commands`（IBM i / ibmi-languages時）
- `ibmi_execute_extension_command`（IBM i / ibmi-languages時）
- `gopls_list_extension_commands`（gopls時）
- `gopls_execute_command`（gopls時）
- `clangd_switch_source_header`（clangd時）
- `clangd_get_symbol_info`（clangd時）
- `ccls_get_call_hierarchy`（ccls時）
- `ccls_get_inheritance_hierarchy`（ccls時）
- `ccls_get_member_hierarchy`（ccls時）
- `ccls_get_vars`（ccls時）
- `ccls_navigate`（ccls時）
- `csharp_list_extension_commands`（csharp系LSP時。OmniSharp / LanguageServer.NET / `csharp-ls` 系）
- `csharp_execute_extension_command`（csharp系LSP時。OmniSharp / LanguageServer.NET / `csharp-ls` 系）
- `qmlls_list_extension_commands`（qmlls時）
- `qmlls_execute_extension_command`（qmlls時）
- `cquery_get_base`（cquery時）
- `cquery_get_derived`（cquery時）
- `cquery_get_callers`（cquery時）
- `cquery_get_vars`（cquery時）
- `cpptools_list_extension_methods`（cpptools時）
- `cpptools_call_extension_method`（cpptools時）
- `cpptools_get_includes`（cpptools時）
- `cpptools_switch_header_source`（cpptools時）

注: `*_list_extension_commands` / `*_list_extension_methods` は接続中サーバーが公開する拡張機能一覧です。実際の引数形式や副作用はサーバー実装依存のため、一覧説明を確認してから `*_execute_*` / `*_call_*` を実行してください。

補足（迷いやすい点）:

- 位置指定は `line` が 1-based、`character` / `column` が UTF-16 の 0-based です。`textTarget` でも位置解決できます。
- `applyEdits` 既定値は、`lsp_format_*` が `false`、`lsp_rename_symbol` が `true` です。
- Go/gopls接続時にコマンド実行するなら、`lsp_execute_command` より `gopls_execute_command` を優先してください（`commandGuide` を返します）。

## 7. 呼び出し例

Hover:

```json
{
  "name": "lsp_get_hover",
  "arguments": {
    "relativePath": "main.go",
    "line": 10,
    "character": 5
  }
}
```

Rename（実ファイル反映）:

```json
{
  "name": "lsp_rename_symbol",
  "arguments": {
    "relativePath": "main.go",
    "line": 10,
    "textTarget": "oldName",
    "newName": "newName",
    "applyEdits": true
  }
}
```

## 8. トラブルシュート

1. `-lsp` が起動しない  
`gopls version` のように単体実行できるか確認してください。

2. `Method not found` が返る  
LSP側がその機能非対応の可能性があります。`lsp_check_capabilities` を実行してください。

3. Windowsでパスエラー  
`-root` は絶対パス推奨です。JSON内の `\` は `\\` でエスケープしてください。
