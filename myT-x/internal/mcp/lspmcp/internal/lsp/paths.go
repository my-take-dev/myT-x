package lsp

import (
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

// PathToURI は絶対パスまたは相対パスを file:// URI に変換する。
func PathToURI(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	slashed := filepath.ToSlash(filepath.Clean(absPath))
	if runtime.GOOS == "windows" && len(slashed) >= 2 && slashed[1] == ':' {
		slashed = "/" + slashed
	}

	u := url.URL{Scheme: "file", Path: slashed}
	return u.String(), nil
}

// URIToPath は file:// URI を OS 固有のパスに変換する。
func URIToPath(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}

	if !strings.EqualFold(u.Scheme, "file") {
		return "", fmt.Errorf("unsupported URI scheme: %s", u.Scheme)
	}

	unescaped, err := url.PathUnescape(u.Path)
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "windows" {
		// file:///C:/path -> /C:/path
		if len(unescaped) >= 3 && unescaped[0] == '/' && unescaped[2] == ':' {
			unescaped = unescaped[1:]
		}
		unescaped = strings.ReplaceAll(unescaped, "/", `\`)

		// UNC path: file://server/share/path
		if u.Host != "" && !strings.HasPrefix(unescaped, `\\`) {
			unescaped = `\\` + u.Host + unescaped
		}
	}

	return filepath.Clean(unescaped), nil
}

// RelativePath は root からの安定した人間可読パスを返す。
// filepath.Rel が失敗した場合（例: 異なるドライブ）は path をそのまま返す。
func RelativePath(rootDir, path string) string {
	if rootDir == "" {
		return path
	}
	rel, err := filepath.Rel(rootDir, path)
	if err != nil {
		log.Printf("[lsp] RelativePath: filepath.Rel(%q, %q) failed: %v, returning path as-is", rootDir, path, err)
		return path
	}
	return rel
}

// DetectLanguageID はファイル拡張子から LSP の languageId を推測する。未対応の拡張子の場合は空文字を返す。
func DetectLanguageID(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c":
		return "c"
	case ".cc", ".cpp", ".cxx":
		return "cpp"
	case ".h", ".hpp":
		return "cpp"
	case ".json":
		return "json"
	case ".yml", ".yaml":
		return "yaml"
	case ".md":
		return "markdown"
	default:
		return ""
	}
}
