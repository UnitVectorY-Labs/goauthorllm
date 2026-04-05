package app

import (
	"strings"
	"testing"

	"goauthorllm/internal/prompts"
)

func TestBuildGenerationMessagesSeparatesInstructionsFromContent(t *testing.T) {
	body := "# Intro\nAlpha line.\n\n# Next\nBeta line."

	messages, err := buildGenerationMessages(body, "Keep terminology stable.", "Prefer short paragraphs.", modeContinue, prompts.Overrides{})
	if err != nil {
		t.Fatalf("build generation messages: %v", err)
	}
	if len(messages) < 5 {
		t.Fatalf("expected multiple messages, got %d", len(messages))
	}

	if messages[0].Role != "system" || messages[0].Name != "system_prompt" {
		t.Fatalf("unexpected system message: %#v", messages[0])
	}

	foundAssistantContent := false
	foundTailContent := false
	foundPromptInstructions := false
	foundDocumentInstructions := false

	for _, message := range messages[1:] {
		switch {
		case message.Name == "document_instructions" && strings.Contains(message.Content, "Keep terminology stable."):
			foundDocumentInstructions = true
		case message.Name == "content" && message.Role == "assistant":
			foundAssistantContent = true
		case message.Name == "content" && message.Role == "user" && strings.Contains(message.Content, "Beta line.") && !strings.Contains(message.Content, "Additional guidance"):
			foundTailContent = true
		case message.Name == "user_instructions" && strings.Contains(message.Content, "Prefer short paragraphs."):
			foundPromptInstructions = true
		}

		if message.Name != "content" && strings.Contains(message.Content, "Beta line.") {
			t.Fatalf("document content leaked into an instructions message: %#v", message)
		}
	}

	if !foundDocumentInstructions {
		t.Fatal("expected separate document instructions message")
	}
	if !foundAssistantContent {
		t.Fatal("expected assistant content messages for prior document sections")
	}
	if !foundTailContent {
		t.Fatal("expected separate user content message for the trailing excerpt")
	}
	if !foundPromptInstructions {
		t.Fatal("expected separate instructions message for additional guidance")
	}
}

func TestBuildEditMessagesSeparatesDocumentContentFromInstructions(t *testing.T) {
	body := "Alpha beta."
	history := []editHistoryEntry{{Action: "skipped", OldText: "Alpha", NewText: "Omega"}}

	messages, err := buildEditMessages(body, "Preserve project terminology.", history, prompts.Overrides{}, "Match must be unique.")
	if err != nil {
		t.Fatalf("build edit messages: %v", err)
	}
	if len(messages) < 4 {
		t.Fatalf("expected multiple edit messages, got %d", len(messages))
	}

	if messages[0].Role != "system" || messages[0].Name != "system_prompt" {
		t.Fatalf("unexpected system message: %#v", messages[0])
	}
	if messages[1].Name != "document_instructions" {
		t.Fatalf("expected document instructions message, got %#v", messages[1])
	}
	if messages[2].Name != "task_instructions" {
		t.Fatalf("expected task instructions message, got %#v", messages[2])
	}
	if messages[3].Name != "content" || messages[3].Role != "user" || messages[3].Content != body {
		t.Fatalf("expected raw document body as separate content message, got %#v", messages[3])
	}

	for i, message := range messages {
		if message.Name != "content" && strings.Contains(message.Content, body) {
			t.Fatalf("body leaked into instructions at index %d: %#v", i, message)
		}
	}
}

func TestBuildGenerationMessagesReturnsPromptRenderErrors(t *testing.T) {
	_, err := buildGenerationMessages("Body", "", "", modeContinue, prompts.Overrides{
		prompts.ContinuePrompt: {
			Replace: "{{.MissingField}}",
		},
	})
	if err == nil {
		t.Fatal("expected prompt render error")
	}
	if !strings.Contains(err.Error(), string(prompts.ContinuePrompt)) {
		t.Fatalf("expected prompt name in error, got %v", err)
	}
}
