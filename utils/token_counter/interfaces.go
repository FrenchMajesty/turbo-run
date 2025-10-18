package token_counter

import "github.com/FrenchMajesty/turbo-run/clients/groq"

type TokenCounterInterface interface {
	CountChatMessagesTokens(messages []groq.ChatMessage) int
	EstimateChatMessageTokens(msg groq.ChatMessage) int
	CountTextTokens(text string) int
	GetTokenCountFromUsage(usage groq.ChatCompletionUsage) (int, int, int)
	CountRequestTokens(request groq.ChatCompletionRequest) int
}
