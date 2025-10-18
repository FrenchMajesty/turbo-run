package groq

type ReasoningEffort string

const (
	ReasoningEffortLow    ReasoningEffort = "low"
	ReasoningEffortMedium ReasoningEffort = "medium"
	ReasoningEffortHigh   ReasoningEffort = "high"
)

type ResponseFormat struct {
	Type       string            `json:"type,omitempty"`
	JsonSchema *JsonSchemaObject `json:"json_schema,omitempty"`
}

type JsonSchemaObject struct {
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Strict      bool                 `json:"strict,omitempty"`
	Schema      JsonSchemaDefinition `json:"schema"`
}

type JsonSchemaType string

const (
	JsonSchemaTypeObject  JsonSchemaType = "object"
	JsonSchemaTypeString  JsonSchemaType = "string"
	JsonSchemaTypeNumber  JsonSchemaType = "number"
	JsonSchemaTypeInteger JsonSchemaType = "integer"
	JsonSchemaTypeBoolean JsonSchemaType = "boolean"
	JsonSchemaTypeArray   JsonSchemaType = "array"
	JsonSchemaTypeNull    JsonSchemaType = "null"
)

type JsonSchemaItemRef string

const (
	JsonSchemaItemRefKey JsonSchemaItemRef = "$ref"
)

type JsonSchemaItemDef string

const (
	JsonSchemaItemDefClassificationHit JsonSchemaItemDef = "classification_hit"
	JsonSchemaItemDefTherapeuticIntent JsonSchemaItemDef = "therapeutic_intent"
	JsonSchemaItemDefFactUpdate        JsonSchemaItemDef = "fact_update"
	JsonSchemaItemDefEmotionUpdate     JsonSchemaItemDef = "emotion_update"
	JsonSchemaItemDefHypothesisUpdate  JsonSchemaItemDef = "hypothesis_update"
	JsonSchemaItemDefConnectionUpdate  JsonSchemaItemDef = "connection_update"
	JsonSchemaItemDefProfileUpdate     JsonSchemaItemDef = "profile_update"
	JsonSchemaItemDefToneNote          JsonSchemaItemDef = "tone_note"
)

type JsonSchemaDefinition struct {
	AnyOf                *[]JsonSchemaDefinition                    `json:"anyOf,omitempty"`
	Type                 JsonSchemaType                             `json:"type,omitempty"`
	Description          string                                     `json:"description,omitempty"`
	Properties           map[string]JsonSchemaDefinition            `json:"properties,omitempty"`
	Required             []string                                   `json:"required,omitempty"`
	AdditionalProperties *bool                                      `json:"additionalProperties,omitempty"`
	Enum                 *[]string                                  `json:"enum,omitempty"`
	Items                map[JsonSchemaItemRef]string               `json:"items,omitempty"` // relevant for array types
	Defs                 map[JsonSchemaItemDef]JsonSchemaDefinition `json:"$defs,omitempty"`
	Nullable             bool                                       `json:"nullable,omitempty"`
}

// ChatCompletionRequest is the request body for the chat completion endpoint
type ChatCompletionRequest struct {
	Model               string            `json:"model"`
	User                string            `json:"user,omitempty"`
	Messages            []ChatMessage     `json:"messages"`
	MaxCompletionTokens int               `json:"max_completion_tokens,omitempty"`
	Temperature         float32           `json:"temperature,omitempty"`
	Tools               *[]ToolDefinition `json:"tools,omitempty"`
	ToolChoice          any               `json:"tool_choice,omitempty"`
	PresencePenalty     float32           `json:"presence_penalty,omitempty"`
	FrequencyPenalty    float32           `json:"frequency_penalty,omitempty"`
	ResponseFormat      *ResponseFormat   `json:"response_format,omitempty"`
	ReasoningEffort     ReasoningEffort   `json:"reasoning_effort,omitempty"`
	Stream              bool              `json:"stream,omitempty"`
	StreamOptions       *StreamOptions    `json:"stream_options,omitempty"`
}

// StreamOptions contains options for streaming responses
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  ToolFunctionParameters `json:"parameters,omitempty"`
}

type ToolFunctionParameters struct {
	Type       string                          `json:"type"`
	Properties map[string]ToolFunctionProperty `json:"properties"`
	Required   []string                        `json:"required"`
}

type ToolFunctionProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}

type ToolCall struct {
	ID      string `json:"id"`
	Tool    string `json:"tool"`
	Message string `json:"message"`
}

type ToolCallRequest struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallResult struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Name    string `json:"name"`
}

type ToolChoiceSelection struct {
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}
