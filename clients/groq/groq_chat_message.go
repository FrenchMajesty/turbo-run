package groq

import (
	"fmt"
	"strings"
)

type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleTool      MessageRole = "tool"
	MessageRoleSystem    MessageRole = "system"
)

type ChatMessage struct {
	Role       MessageRole        `json:"role"`
	Content    *string            `json:"content,omitempty"`
	Reasoning  *string            `json:"reasoning,omitempty"`
	ToolCalls  *[]ToolCallRequest `json:"tool_calls,omitempty"`
	ToolCallID *string            `json:"tool_call_id,omitempty"`
}

// ToPlainText formats the message as a plain text string
func (m *ChatMessage) ToPlainText() string {
	content := ""
	if m.Content != nil {
		content = *m.Content
	}

	return fmt.Sprintf("%s: %s", m.Role, content)
}

// DeepCopy creates a deep copy of ChatMessage
func (m *ChatMessage) DeepCopy() ChatMessage {
	copied := ChatMessage{
		Role: m.Role,
	}

	// Copy Content pointer
	if m.Content != nil {
		content := *m.Content
		copied.Content = &content
	}

	// Copy ToolCallID pointer
	if m.ToolCallID != nil {
		toolCallID := *m.ToolCallID
		copied.ToolCallID = &toolCallID
	}

	// Deep copy ToolCalls slice
	if m.ToolCalls != nil {
		toolCalls := make([]ToolCallRequest, len(*m.ToolCalls))
		copy(toolCalls, *m.ToolCalls)
		copied.ToolCalls = &toolCalls
	}

	return copied
}

// IsToolMessage checks if the message is a tool message (trigger or response)
func (m *ChatMessage) IsToolMessage() bool {
	isToolCallJsonString := m.Content != nil && strings.HasPrefix(*m.Content, "[{")
	hasToolCallsArray := m.ToolCalls != nil && len(*m.ToolCalls) > 0
	isToolResponse := m.Role == MessageRoleTool || m.ToolCallID != nil
	return hasToolCallsArray || isToolResponse || isToolCallJsonString
}

// IsSystemMessage checks if the message is a system message
func (m *ChatMessage) IsSystemMessage() bool {
	return m.Role == MessageRoleSystem
}
