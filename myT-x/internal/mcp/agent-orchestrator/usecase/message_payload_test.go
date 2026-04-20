package usecase

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

func TestPayloadWriterPrepareTaskMessageStorageBoundaries(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	writer := newPayloadWriter(projectRoot)

	inlineContent := strings.Repeat("a", MaxInlineDeliveryChars)
	inlinePayload, err := writer.PrepareTaskMessage("t-inline", "m-inline", inlineContent, "2026-04-18T01:02:03Z")
	if err != nil {
		t.Fatalf("PrepareTaskMessage(inline): %v", err)
	}
	if inlinePayload.message.StorageMode != domain.MessageStorageInline {
		t.Fatalf("inline storage_mode = %q, want %q", inlinePayload.message.StorageMode, domain.MessageStorageInline)
	}
	if inlinePayload.deliveryText != inlineContent {
		t.Fatalf("inline delivery text = %q, want original content", inlinePayload.deliveryText)
	}

	fileContent := strings.Repeat("b", MaxInlineDeliveryChars+1)
	filePayload, err := writer.PrepareTaskMessage("t-file", "m-file", fileContent, "2026-04-18T01:02:03Z")
	if err != nil {
		t.Fatalf("PrepareTaskMessage(file): %v", err)
	}
	if filePayload.message.StorageMode != domain.MessageStorageFile {
		t.Fatalf("file storage_mode = %q, want %q", filePayload.message.StorageMode, domain.MessageStorageFile)
	}
	if len(filePayload.message.ArtifactPaths) != 1 {
		t.Fatalf("file artifact_paths len = %d, want 1", len(filePayload.message.ArtifactPaths))
	}
	wantRelativePath := filepath.ToSlash(filepath.Join(payloadRelativeDir, "t-file__m-file.md"))
	if got := filePayload.message.ArtifactPaths[0]; got != wantRelativePath {
		t.Fatalf("file artifact path = %q, want %q", got, wantRelativePath)
	}
	wantAbsolutePath := filepath.Join(projectRoot, filepath.FromSlash(wantRelativePath))
	if !strings.Contains(filePayload.deliveryText, "payload_path="+wantAbsolutePath) {
		t.Fatalf("file delivery text = %q, want resolved payload_path", filePayload.deliveryText)
	}
}

func TestPayloadWriterPrepareTaskMessageMultipartManifestUsesRelativeStoredPaths(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	writer := newPayloadWriter(projectRoot)
	content := strings.Repeat("x", payloadMultipartThresholdB+1)

	payload, err := writer.PrepareTaskMessage("t-multipart", "m-multipart", content, "2026-04-18T01:02:03Z")
	if err != nil {
		t.Fatalf("PrepareTaskMessage(multipart): %v", err)
	}
	if payload.message.StorageMode != domain.MessageStorageMultipartFile {
		t.Fatalf("storage_mode = %q, want %q", payload.message.StorageMode, domain.MessageStorageMultipartFile)
	}
	if len(payload.message.ArtifactPaths) < 2 {
		t.Fatalf("artifact_paths len = %d, want manifest + parts", len(payload.message.ArtifactPaths))
	}

	manifestRelativePath := filepath.ToSlash(filepath.Join(payloadRelativeDir, "t-multipart__m-multipart__manifest.json"))
	if got := payload.message.ArtifactPaths[0]; got != manifestRelativePath {
		t.Fatalf("manifest artifact path = %q, want %q", got, manifestRelativePath)
	}
	manifestPath := filepath.Join(projectRoot, filepath.FromSlash(manifestRelativePath))
	if !strings.Contains(payload.deliveryText, "payload_manifest="+manifestPath) {
		t.Fatalf("delivery text = %q, want resolved payload_manifest", payload.deliveryText)
	}

	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", manifestPath, err)
	}
	var manifest payloadManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("Unmarshal(manifest): %v", err)
	}
	if manifest.PartCount != payload.message.PartCount {
		t.Fatalf("manifest part_count = %d, want %d", manifest.PartCount, payload.message.PartCount)
	}
	if len(manifest.Parts) != payload.message.PartCount {
		t.Fatalf("manifest parts len = %d, want %d", len(manifest.Parts), payload.message.PartCount)
	}
	for _, partPath := range manifest.Parts {
		if filepath.IsAbs(partPath) {
			t.Fatalf("manifest part path = %q, want project-relative path", partPath)
		}
	}
}

func TestDeliveryTextForStoredMessageIncludesResponseInstructions(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	message := domain.TaskMessage{
		ID:             "m-001",
		StorageMode:    domain.MessageStorageFile,
		ContentPreview: "stored preview",
		ArtifactPaths:  []string{filepath.ToSlash(filepath.Join(payloadRelativeDir, "t-001__m-001.md"))},
		PartCount:      1,
		ContentChars:   MaxInlineDeliveryChars + 1,
		SHA256:         "abc123",
	}

	deliveryText := deliveryTextForStoredMessage(projectRoot, "t-001", message)
	if !strings.Contains(deliveryText, "payload_path="+filepath.Join(projectRoot, ".myT-x", "orchestrator", "payloads", "t-001__m-001.md")) {
		t.Fatalf("delivery text = %q, want resolved payload path", deliveryText)
	}
	if !strings.Contains(deliveryText, "send_response(task_id=\"t-001\", message=\"...\")") {
		t.Fatalf("delivery text = %q, want response instructions", deliveryText)
	}
}
