package entire

import (
	"encoding/json"
	"errors"
)

// ErrTranscriptUnavailable reports a transcript that Entire could not read,
// such as when its stored copy is missing.
var ErrTranscriptUnavailable = errors.New("entire: transcript unavailable")

// Transcript is an agent session's conversation: the prompts, the agent's
// replies, and the tools it called.
type Transcript struct {
	// SessionID identifies the session the transcript belongs to.
	SessionID string `json:"sessionId"`
	// Agent is the coding agent that recorded the transcript, such as
	// "Claude Code" or "Codex". It is "Claude Code" when the session
	// recorded no agent.
	Agent string `json:"agent"`
	// Messages are the conversation's messages, in order.
	Messages []TranscriptMessage `json:"messages"`
	// TokenUsage counts the tokens the session used. For a session's merged
	// transcript it totals the session's checkpoints; for a checkpoint's
	// transcript it covers that checkpoint. It is zero when the session
	// reported no usage.
	TokenUsage TokenUsage `json:"tokenUsage"`
	// Attribution measures how much of the committed code the agent wrote,
	// or is nil when it is unknown. For a session's merged transcript it is
	// the attribution of the session's latest checkpoint.
	Attribution *Attribution `json:"attribution"`
	// ParsingWarning explains a transcript that Entire could only partly
	// parse, or that it truncated. It is empty when the transcript parsed
	// fully.
	ParsingWarning string `json:"parsingWarning"`
}

// TranscriptMessage is one message of a transcript.
type TranscriptMessage struct {
	// ID identifies the message within its transcript.
	ID string `json:"id"`
	// Role is who sent the message: "user" or "assistant".
	Role string `json:"role"`
	// Content is the message's text.
	Content string `json:"content"`
	// Timestamp is when the message was sent, or the zero time when the
	// agent recorded no time that parses as RFC 3339.
	Timestamp Timestamp `json:"timestamp"`
	// ToolCalls are the tools that the message called, in order. It is
	// empty for a message that called none.
	ToolCalls []ToolCall `json:"toolCalls"`
}

// ToolCall is a tool that an agent called.
type ToolCall struct {
	// ID identifies the call, as the agent recorded it.
	ID string `json:"id"`
	// Name is the tool, such as "Bash" or "Edit".
	Name string `json:"name"`
	// Input is the call's arguments, as a JSON object.
	Input json.RawMessage `json:"input"`
	// Result is what the call returned, or nil when the transcript recorded
	// no result.
	Result *ToolResult `json:"result"`
	// Timestamp is when the tool was called, or the zero time when the
	// agent recorded no time that parses as RFC 3339.
	Timestamp Timestamp `json:"timestamp"`
	// Categories are the timeline filters the call belongs to, such as
	// ToolCategoryBash. Entire may add categories, so expect values the SDK
	// does not name.
	Categories []ToolCategory `json:"categories"`
	// Operations are the tools that ran inside this call, in order, each
	// with its own result, such as the commands a Codex exec script ran. It
	// is empty when the agent recorded none, and may be incomplete, because
	// agents do not log every tool.
	Operations []ToolCall `json:"operations"`
}

// ToolCategory is a timeline filter that a tool call belongs to.
type ToolCategory string

// Tool call categories.
const (
	ToolCategoryFileEdits ToolCategory = "fileEdits"
	ToolCategoryBash      ToolCategory = "bash"
	ToolCategoryRead      ToolCategory = "read"
	ToolCategorySkills    ToolCategory = "skills"
	ToolCategoryEntire    ToolCategory = "entire" // calls to the Entire CLI or its tools
	ToolCategoryOther     ToolCategory = "other"
)

// ToolResult is what a tool call returned.
type ToolResult struct {
	// Type is the kind of result, such as "text" or "error". It is "error"
	// when the call failed.
	Type string `json:"type"`
	// Output is the text that the tool returned. It may be empty, even for
	// a call that succeeded.
	Output string `json:"output"`
	// Error is the failure message when Type is "error", and is empty
	// otherwise.
	Error string `json:"error"`
}

// TokenUsage counts the tokens that an agent session used.
type TokenUsage struct {
	// InputTokens counts the tokens sent to the model.
	InputTokens int64 `json:"inputTokens"`
	// OutputTokens counts the tokens the model generated.
	OutputTokens int64 `json:"outputTokens"`
}

// Attribution measures how much of the committed code an agent wrote, as
// opposed to people.
type Attribution struct {
	// AgentLines counts the lines the agent added that remain in the
	// commit.
	AgentLines int64 `json:"agentLines"`
	// TotalCommitted counts the lines that the commit adds, net.
	TotalCommitted int64 `json:"totalCommitted"`
	// AgentPercentage is the percentage, from 0 to 100, of the committed
	// line changes that the agent made. Older versions of the Entire CLI
	// computed it as AgentLines as a percentage of TotalCommitted.
	AgentPercentage float64 `json:"agentPercentage"`
	// HumanAdded counts the lines that people added, not counting lines
	// they modified.
	HumanAdded int64 `json:"humanAdded"`
	// HumanModified estimates the lines that people changed.
	HumanModified int64 `json:"humanModified"`
	// HumanRemoved counts the lines that people removed, not counting
	// lines they modified.
	HumanRemoved int64 `json:"humanRemoved"`
	// CalculatedAt is when the attribution was calculated.
	CalculatedAt Timestamp `json:"calculatedAt"`
}
