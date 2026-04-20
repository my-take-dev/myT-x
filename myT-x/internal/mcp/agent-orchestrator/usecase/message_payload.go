package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

const (
	MaxAcceptedMessageChars      = 200000
	MaxInlineDeliveryChars       = 16000
	payloadPreviewChars          = 240
	payloadMultipartThresholdB   = 256 * 1024
	payloadMultipartPartMaxBytes = 128 * 1024
	payloadRelativeDir           = ".myT-x/orchestrator/payloads"
)

type preparedPayload struct {
	message      domain.TaskMessage
	deliveryText string
	cleanupPaths []string
}

type payloadWriter struct {
	projectRoot string
}

type payloadManifest struct {
	TaskID       string   `json:"task_id"`
	PayloadID    string   `json:"payload_id"`
	StorageMode  string   `json:"storage_mode"`
	PartCount    int      `json:"part_count"`
	ContentChars int      `json:"content_chars"`
	SHA256       string   `json:"sha256"`
	Parts        []string `json:"parts"`
}

func newPayloadWriter(projectRoot string) *payloadWriter {
	root := filepath.Clean(strings.TrimSpace(projectRoot))
	if root == "" {
		return nil
	}
	return &payloadWriter{projectRoot: root}
}

func (w *payloadWriter) PrepareTaskMessage(taskID, messageID, content, createdAt string) (preparedPayload, error) {
	return w.prepare(taskID, messageID, content, createdAt)
}

func (w *payloadWriter) PrepareResponse(taskID, responseID, content, createdAt string) (preparedPayload, error) {
	return w.prepare(taskID, responseID, content, createdAt)
}

func (w *payloadWriter) prepare(taskID, payloadID, content, createdAt string) (preparedPayload, error) {
	preview := buildMessagePreview(content)
	contentChars := utf8.RuneCountInString(content)
	sha256Value := sha256Hex(content)
	if contentChars <= MaxInlineDeliveryChars {
		return preparedPayload{
			message: domain.TaskMessage{
				ID:             payloadID,
				Content:        content,
				CreatedAt:      createdAt,
				StorageMode:    domain.MessageStorageInline,
				ContentPreview: preview,
				ContentChars:   contentChars,
				SHA256:         sha256Value,
			},
			deliveryText: content,
		}, nil
	}
	if w == nil || w.projectRoot == "" {
		return preparedPayload{}, fmt.Errorf("payload writer is unavailable for spillover content")
	}

	if len(content) > payloadMultipartThresholdB {
		return w.prepareMultipart(taskID, payloadID, content, createdAt, preview, contentChars, sha256Value)
	}
	return w.prepareFile(taskID, payloadID, content, createdAt, preview, contentChars, sha256Value)
}

func (w *payloadWriter) prepareFile(
	taskID string,
	payloadID string,
	content string,
	createdAt string,
	preview string,
	contentChars int,
	sha256Value string,
) (preparedPayload, error) {
	artifactPath := filepath.ToSlash(filepath.Join(payloadRelativeDir, fmt.Sprintf("%s__%s.md", taskID, payloadID)))
	absPath := filepath.Join(w.projectRoot, filepath.FromSlash(artifactPath))
	if err := writeFileAtomically(absPath, []byte(content)); err != nil {
		return preparedPayload{}, fmt.Errorf("write payload file %q: %w", artifactPath, err)
	}
	message := domain.TaskMessage{
		ID:             payloadID,
		CreatedAt:      createdAt,
		StorageMode:    domain.MessageStorageFile,
		ContentPreview: preview,
		ArtifactPaths:  []string{artifactPath},
		PartCount:      1,
		ContentChars:   contentChars,
		SHA256:         sha256Value,
	}
	return preparedPayload{
		message:      message,
		deliveryText: buildPayloadNotification(taskID, w.projectRoot, message),
		cleanupPaths: []string{absPath},
	}, nil
}

func (w *payloadWriter) prepareMultipart(
	taskID string,
	payloadID string,
	content string,
	createdAt string,
	preview string,
	contentChars int,
	sha256Value string,
) (preparedPayload, error) {
	chunks := splitStringByByteLimit(content, payloadMultipartPartMaxBytes)
	if len(chunks) == 0 {
		return preparedPayload{}, fmt.Errorf("multipart payload split returned no chunks")
	}

	cleanupPaths := make([]string, 0, len(chunks)+1)
	partArtifactPaths := make([]string, 0, len(chunks))
	for index, chunk := range chunks {
		partArtifactPath := filepath.ToSlash(filepath.Join(
			payloadRelativeDir,
			fmt.Sprintf("%s__%s__p%03d-of%03d.md", taskID, payloadID, index+1, len(chunks)),
		))
		partAbsPath := filepath.Join(w.projectRoot, filepath.FromSlash(partArtifactPath))
		if err := writeFileAtomically(partAbsPath, []byte(chunk)); err != nil {
			cleanupErr := cleanupPayloadArtifacts(cleanupPaths)
			if cleanupErr != nil {
				return preparedPayload{}, fmt.Errorf("write multipart payload %q: %w (cleanup: %v)", partArtifactPath, err, cleanupErr)
			}
			return preparedPayload{}, fmt.Errorf("write multipart payload %q: %w", partArtifactPath, err)
		}
		partArtifactPaths = append(partArtifactPaths, partArtifactPath)
		cleanupPaths = append(cleanupPaths, partAbsPath)
	}

	manifestArtifactPath := filepath.ToSlash(filepath.Join(payloadRelativeDir, fmt.Sprintf("%s__%s__manifest.json", taskID, payloadID)))
	manifestAbsPath := filepath.Join(w.projectRoot, filepath.FromSlash(manifestArtifactPath))
	manifestBytes, err := json.MarshalIndent(payloadManifest{
		TaskID:       taskID,
		PayloadID:    payloadID,
		StorageMode:  string(domain.MessageStorageMultipartFile),
		PartCount:    len(partArtifactPaths),
		ContentChars: contentChars,
		SHA256:       sha256Value,
		Parts:        append([]string(nil), partArtifactPaths...),
	}, "", "  ")
	if err != nil {
		cleanupErr := cleanupPayloadArtifacts(cleanupPaths)
		if cleanupErr != nil {
			return preparedPayload{}, fmt.Errorf("marshal multipart manifest: %w (cleanup: %v)", err, cleanupErr)
		}
		return preparedPayload{}, fmt.Errorf("marshal multipart manifest: %w", err)
	}
	manifestBytes = append(manifestBytes, '\n')
	if err := writeFileAtomically(manifestAbsPath, manifestBytes); err != nil {
		cleanupErr := cleanupPayloadArtifacts(cleanupPaths)
		if cleanupErr != nil {
			return preparedPayload{}, fmt.Errorf("write manifest %q: %w (cleanup: %v)", manifestArtifactPath, err, cleanupErr)
		}
		return preparedPayload{}, fmt.Errorf("write manifest %q: %w", manifestArtifactPath, err)
	}
	cleanupPaths = append(cleanupPaths, manifestAbsPath)
	artifactPaths := append([]string{manifestArtifactPath}, partArtifactPaths...)

	message := domain.TaskMessage{
		ID:             payloadID,
		CreatedAt:      createdAt,
		StorageMode:    domain.MessageStorageMultipartFile,
		ContentPreview: preview,
		ArtifactPaths:  artifactPaths,
		PartCount:      len(partArtifactPaths),
		ContentChars:   contentChars,
		SHA256:         sha256Value,
	}
	return preparedPayload{
		message:      message,
		deliveryText: buildPayloadNotification(taskID, w.projectRoot, message),
		cleanupPaths: cleanupPaths,
	}, nil
}

func (p preparedPayload) Cleanup() error {
	return cleanupPayloadArtifacts(p.cleanupPaths)
}

func cleanupPayloadArtifacts(paths []string) error {
	var errs []error
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove %q: %w", path, err))
		}
	}
	return errors.Join(errs...)
}

func buildPayloadNotification(taskID string, projectRoot string, message domain.TaskMessage) string {
	var b strings.Builder
	storageMode := domain.NormalizeMessageStorageMode(message.StorageMode)
	artifactPaths := domain.ResolveArtifactPaths(projectRoot, message.ArtifactPaths)

	b.WriteString("task_id=")
	b.WriteString(taskID)
	b.WriteString("\n")
	b.WriteString("payload_mode=")
	b.WriteString(string(storageMode))
	b.WriteString("\n")

	switch storageMode {
	case domain.MessageStorageMultipartFile:
		if len(artifactPaths) > 0 {
			b.WriteString("payload_manifest=")
			b.WriteString(artifactPaths[0])
			b.WriteString("\n")
		}
		if message.PartCount > 0 {
			b.WriteString("part_count=")
			b.WriteString(fmt.Sprintf("%d", message.PartCount))
			b.WriteString("\n")
		}
		b.WriteString("Read the manifest file, then continue the task.")
	default:
		if len(artifactPaths) > 0 {
			b.WriteString("payload_path=")
			b.WriteString(artifactPaths[0])
			b.WriteString("\n")
		}
		b.WriteString("Read the payload file, then continue the task.")
	}
	return b.String()
}

func deliveryTextForStoredMessage(projectRoot string, taskID string, message domain.TaskMessage) string {
	if domain.NormalizeMessageStorageMode(message.StorageMode) == domain.MessageStorageInline {
		return message.Content
	}
	return appendResponseInstruction(buildPayloadNotification(taskID, projectRoot, message), taskID)
}

func buildMessagePreview(content string) string {
	runes := []rune(content)
	if len(runes) <= payloadPreviewChars {
		return content
	}
	return string(runes[:payloadPreviewChars])
}

func sha256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func splitStringByByteLimit(content string, maxBytes int) []string {
	if content == "" {
		return nil
	}
	if maxBytes <= 0 || len(content) <= maxBytes {
		return []string{content}
	}

	chunks := make([]string, 0, (len(content)/maxBytes)+1)
	start := 0
	currentBytes := 0
	for index, r := range content {
		runeBytes := utf8.RuneLen(r)
		if runeBytes < 0 {
			runeBytes = 1
		}
		if currentBytes+runeBytes > maxBytes && index > start {
			chunks = append(chunks, content[start:index])
			start = index
			currentBytes = 0
		}
		currentBytes += runeBytes
	}
	if start < len(content) {
		chunks = append(chunks, content[start:])
	}
	return chunks
}

func writeFileAtomically(path string, content []byte) (retErr error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create payload directory: %w", err)
	}

	tempFile, err := os.CreateTemp(dir, ".payload-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		if tempFile != nil {
			_ = tempFile.Close()
		}
		if retErr != nil {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(content); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	tempFile = nil
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
