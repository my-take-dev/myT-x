package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"myT-x/internal/ipc"
)

const (
	shimDebugLogFileName        = "shim-debug.log"
	shimDebugLogMaxBytes        = 5 * 1024 * 1024
	shimDebugLogKeepGenerations = 32 // Approx. max usage: shimDebugLogKeepGenerations * shimDebugLogMaxBytes.
	debugLogFallbackMaxMessages = 3
)

var (
	// Keep only the first fallback reason to avoid flooding stderr.
	// Message count throttling is handled separately by debugLogFallbackMessageCount.
	debugLogFallbackMu           sync.Mutex
	debugLogFallbackLogged       bool
	debugLogFallbackMessageCount int
	renameFileFn                 = os.Rename
	removeFileFn                 = os.Remove
	pruneCountByDirMu            sync.Mutex
	// Cache per-process rotated log counts to avoid repeated directory scans.
	pruneCountByDir = map[string]int{}
)

// debugLog writes shim debug info to a log file for troubleshooting.
// Active log file: %LOCALAPPDATA%\myT-x\shim-debug.log
// Rotated log file: %LOCALAPPDATA%\myT-x\shim-debug-<unixtime>.log
func debugLog(format string, args ...any) {
	message := fmt.Sprintf(format, args...)

	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		debugLogFallback(errors.New("LOCALAPPDATA is empty"))
		debugLogFallbackMessage(message)
		return
	}
	logDir := filepath.Join(localAppData, "myT-x")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		debugLogFallback(fmt.Errorf("create log directory %q: %w", logDir, err))
		debugLogFallbackMessage(message)
		return
	}
	logPath := filepath.Join(logDir, shimDebugLogFileName)
	if err := rotateShimDebugLogIfNeeded(logPath, shimDebugLogMaxBytes, time.Now().Unix()); err != nil {
		debugLogFallback(fmt.Errorf("rotate log file %q: %w", logPath, err))
		debugLogFallbackMessage(message)
		return
	}
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		debugLogFallback(fmt.Errorf("open log file %q: %w", logPath, err))
		debugLogFallbackMessage(message)
		return
	}
	defer f.Close()
	logger := log.New(f, "[DEBUG-SHIM] ", log.LstdFlags)
	logger.Print(message)
}

func debugLogFallback(err error) {
	if err == nil {
		return
	}
	debugLogFallbackMu.Lock()
	if debugLogFallbackLogged {
		debugLogFallbackMu.Unlock()
		return
	}
	debugLogFallbackLogged = true
	debugLogFallbackMu.Unlock()
	// NOTE: stderr fallback logging is best-effort for debug visibility only.
	// Message ordering is not guaranteed across concurrent writers.
	if _, writeErr := fmt.Fprintf(os.Stderr, "[DEBUG-SHIM] logging unavailable: %v\n", err); writeErr != nil {
		// S-11: Practically unreachable — stderr would need to be closed or redirected
		// to a broken pipe for this branch to execute. Kept as defense-in-depth to avoid
		// panicking if the OS environment is severely degraded.
	}
}

func debugLogFallbackMessage(message string) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return
	}

	debugLogFallbackMu.Lock()
	if debugLogFallbackMessageCount >= debugLogFallbackMaxMessages {
		debugLogFallbackMu.Unlock()
		return
	}
	debugLogFallbackMessageCount++
	debugLogFallbackMu.Unlock()

	log.New(os.Stderr, "[DEBUG-SHIM] ", log.LstdFlags).Print(trimmed)
}

func flushDebugLogFallbackSummary() {
	// no-op: fallback messages are already bounded by debugLogFallbackMaxMessages.
}

func exitWithCode(code int) {
	flushDebugLogFallbackSummary()
	os.Exit(code)
}

func main() {
	args := os.Args[1:]
	debugLog("invoked: tmux %s", strings.Join(args, " "))

	if len(args) == 0 {
		printUsage()
		flushDebugLogFallbackSummary()
		return
	}

	req, err := parseCommand(args)
	if err != nil {
		debugLog("parse error: %v (args=%v)", err, args)
		writeLineToStderr(err.Error())
		exitWithCode(1)
	}

	debugLog("parsed: command=%s flags=%s env=%v args=%v",
		req.Command, flagsJSON(req.Flags), req.Env, req.Args)
	debugLog("received request before transform: %s", requestJSON(req))

	shellChanged, shellErr := runTransformSafe("shell", &req, func() (bool, error) {
		return applyShellTransform(&req), nil
	})
	if shellErr != nil {
		debugLog("shell transform skipped: %v", shellErr)
	} else if shellChanged {
		debugLog("shell transform applied: command=%s flags=%s env=%v args=%v",
			req.Command, flagsJSON(req.Flags), req.Env, req.Args)
	}

	req.CallerPane = strings.TrimSpace(os.Getenv("TMUX_PANE"))
	// NOTE: applyModelTransform always returns nil error (config failures are swallowed per shim spec).
	// transformErr is non-nil only when runTransformSafe recovers from a panic — handled below.
	transformed, transformErr := runTransformSafe("model", &req, func() (bool, error) {
		return applyModelTransform(&req, nil)
	})
	if transformErr != nil {
		debugLog("model transform skipped: %v", transformErr)
	} else if transformed {
		debugLog("model transform applied: command=%s args=%v", req.Command, req.Args)
	}
	debugLog("sending request after transform: %s", requestJSON(req))

	pipeName := ipc.DefaultPipeName()

	resp, err := ipc.Send(pipeName, req)
	if err != nil {
		debugLog("ipc error: %v", err)
		if ipc.IsConnectionError(err) {
			writeToStderr("no server running on %s\n", pipeName)
			exitWithCode(1)
		}
		writeLineToStderr(err.Error())
		exitWithCode(1)
	}

	debugLog("response: exit=%d stdout=%q stderr=%q",
		resp.ExitCode, truncate(resp.Stdout, 200), truncate(resp.Stderr, 200))

	if resp.Stdout != "" {
		writeToStdout(resp.Stdout)
	}
	if resp.Stderr != "" {
		writeToStderr("%s", resp.Stderr)
	}
	exitWithCode(resp.ExitCode)
}

func flagsJSON(flags map[string]any) string {
	b, err := json.Marshal(flags)
	if err != nil {
		return fmt.Sprintf("%v", flags)
	}
	return string(b)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func requestJSON(req ipc.TmuxRequest) string {
	raw, err := json.Marshal(req)
	if err != nil {
		return fmt.Sprintf("command=%s flags=%v args=%v env=%v callerPane=%s",
			req.Command, req.Flags, req.Args, req.Env, req.CallerPane)
	}
	return string(raw)
}

func writeToStdout(message string) {
	if _, err := fmt.Fprint(os.Stdout, message); err != nil {
		debugLog("stdout write failed: %v", err)
	}
}

func writeToStderr(format string, args ...any) {
	if _, err := fmt.Fprintf(os.Stderr, format, args...); err != nil {
		debugLog("stderr write failed: %v", err)
	}
}

func writeLineToStderr(message string) {
	writeToStderr("%s\n", message)
}

func rotateShimDebugLogIfNeeded(basePath string, maxBytes int64, unixTime int64) error {
	info, err := os.Stat(basePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Size() < maxBytes {
		return nil
	}

	logDir := filepath.Dir(basePath)
	for retry := range 4 {
		nextPath, err := nextRotatedShimDebugLogPath(logDir, unixTime+int64(retry))
		if err != nil {
			return err
		}
		err = renameFileFn(basePath, nextPath)
		if err == nil {
			if shouldPruneRotatedShimDebugLogs(logDir, shimDebugLogKeepGenerations) {
				if cleanupErr := pruneRotatedShimDebugLogs(logDir, shimDebugLogKeepGenerations); cleanupErr != nil {
					debugLogFallback(fmt.Errorf("prune rotated log files in %q: %w", logDir, cleanupErr))
				} else {
					markPrunedRotatedShimDebugLogs(logDir, shimDebugLogKeepGenerations)
				}
			}
			return nil
		}
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if errors.Is(err, os.ErrNotExist) {
			// Another process already rotated/deleted it.
			return nil
		}
		return err
	}
	return fmt.Errorf("failed to rotate log file after retries: %s", basePath)
}

func nextRotatedShimDebugLogPath(logDir string, unixTime int64) (string, error) {
	// keep=32 is the normal steady state, and 64 keeps 2x headroom for
	// short timestamp collisions during concurrent rotations.
	const maxAttempts = 64
	for offset := range int64(maxAttempts) {
		candidateUnix := unixTime + offset
		candidate := filepath.Join(logDir, fmt.Sprintf("shim-debug-%d.log", candidateUnix))
		_, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("unable to allocate rotated log path from unix=%d", unixTime)
}

// shouldPruneRotatedShimDebugLogs updates the per-directory generation count
// and reports whether pruneRotatedShimDebugLogs should run.
func shouldPruneRotatedShimDebugLogs(logDir string, keep int) bool {
	if keep <= 0 {
		return false
	}

	pruneCountByDirMu.Lock()
	defer pruneCountByDirMu.Unlock()
	if count, ok := pruneCountByDir[logDir]; ok {
		count++
		pruneCountByDir[logDir] = count
		return count > keep
	}

	candidateCount, err := countRotatedShimDebugLogs(logDir)
	if err != nil {
		return true
	}

	pruneCountByDir[logDir] = candidateCount
	return candidateCount > keep
}

func markPrunedRotatedShimDebugLogs(logDir string, keep int) {
	if keep < 0 {
		keep = 0
	}
	pruneCountByDirMu.Lock()
	pruneCountByDir[logDir] = keep
	pruneCountByDirMu.Unlock()
}

// countRotatedShimDebugLogs returns the number of valid shim-debug-<unix>.log
// files in logDir.
func countRotatedShimDebugLogs(logDir string) (int, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, ok := parseRotatedShimDebugLogUnix(entry.Name()); ok {
			count++
		}
	}
	return count, nil
}

// parseRotatedShimDebugLogUnix parses shim-debug-<unix>.log and returns its
// unix timestamp.
func parseRotatedShimDebugLogUnix(path string) (int64, bool) {
	name := filepath.Base(path)
	if !strings.HasPrefix(name, "shim-debug-") || !strings.HasSuffix(name, ".log") {
		return 0, false
	}
	timestampText := strings.TrimSuffix(strings.TrimPrefix(name, "shim-debug-"), ".log")
	timestamp, err := strconv.ParseInt(timestampText, 10, 64)
	if err != nil {
		return 0, false
	}
	return timestamp, true
}

// pruneRotatedShimDebugLogs keeps the newest keep generations and removes
// older rotated shim logs.
//
// IMPORTANT: This function MUST NOT call debugLog() because it is invoked from
// the rotation path: debugLog -> rotateShimDebugLogIfNeeded -> pruneRotatedShimDebugLogs.
// Calling debugLog here would create infinite recursion -> stack overflow (C-03).
// Use pruneLogWarning() for any diagnostic output instead.
func pruneRotatedShimDebugLogs(logDir string, keep int) error {
	if keep <= 0 {
		return nil
	}

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return err
	}

	type rotatedLog struct {
		path      string
		timestamp int64
	}
	logs := make([]rotatedLog, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		timestamp, ok := parseRotatedShimDebugLogUnix(name)
		if !ok {
			if strings.HasPrefix(name, "shim-debug-") && strings.HasSuffix(name, ".log") {
				// Cannot use debugLog here — see function doc comment (C-03 recursion guard).
				pruneLogWarning("skip rotated shim debug log with invalid unix timestamp: %s", name)
			}
			continue
		}

		logs = append(logs, rotatedLog{
			path:      filepath.Join(logDir, name),
			timestamp: timestamp,
		})
	}

	sort.Slice(logs, func(i, j int) bool {
		return logs[i].timestamp > logs[j].timestamp
	})

	var removeErrs []error
	for i := keep; i < len(logs); i++ {
		if err := removeFileFn(logs[i].path); err != nil && !errors.Is(err, os.ErrNotExist) {
			removeErrs = append(removeErrs, fmt.Errorf("remove %s: %w", logs[i].path, err))
		}
	}
	return errors.Join(removeErrs...)
}

// pruneLogWarning writes a warning directly to stderr, bypassing debugLog
// to avoid infinite recursion in the log rotation path (C-03).
// This is intentionally best-effort: if stderr is unavailable the warning is silently dropped.
// Uses fmt.Fprintf instead of log.New to avoid allocating a new Logger per call.
func pruneLogWarning(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	now := time.Now().Format("2006/01/02 15:04:05")
	// NOTE: stderr write is best-effort for debug visibility only.
	// Failure to write is silently ignored (shim must not block on diagnostics).
	fmt.Fprintf(os.Stderr, "[DEBUG-SHIM] %s %s\n", now, msg)
}

// runTransformSafe executes one transform stage with panic recovery and request rollback.
// When an error or panic occurs, the original request snapshot is restored.
func runTransformSafe(name string, req *ipc.TmuxRequest, run func() (bool, error)) (changed bool, err error) {
	if req == nil {
		return false, errors.New("tmux request is nil")
	}
	snapshot := cloneTransformRequest(req)

	defer func() {
		if recovered := recover(); recovered != nil {
			debugLog("panic recovered in %s transform: %v\n%s", name, recovered, debug.Stack())
			err = fmt.Errorf("panic during %s transform: %v", name, recovered)
		}
		if err != nil {
			if changed {
				debugLog("%s transform returned changed=true with error; restoring snapshot: %v", name, err)
			}
			*req = snapshot
			changed = false
		}
	}()

	changed, err = run()
	return changed, err
}

// cloneTransformRequest creates a rollback snapshot for transform safety checks.
// Flags values are expected to be string or bool; a map-level clone is sufficient.
func cloneTransformRequest(req *ipc.TmuxRequest) ipc.TmuxRequest {
	if req == nil {
		return ipc.TmuxRequest{}
	}

	clone := *req
	if req.Flags != nil {
		clone.Flags = make(map[string]any, len(req.Flags))
		maps.Copy(clone.Flags, req.Flags)
	}
	if req.Env != nil {
		clone.Env = make(map[string]string, len(req.Env))
		maps.Copy(clone.Env, req.Env)
	}
	if req.Args != nil {
		clone.Args = append([]string(nil), req.Args...)
	}
	return clone
}
