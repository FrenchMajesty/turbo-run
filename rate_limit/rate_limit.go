package rate_limit

// RateLimit defines token per minute (TPM) and request per minute (RPM) limits
type RateLimit struct {
	RPM int // Requests per minute
	TPM int // Tokens per minute
}

// GroqRateLimit defines the default rate limits for Groq API
var GroqRateLimit = RateLimit{
	RPM: 1 * 1000,          // 1K RPM
	TPM: (400 * 1000) * .9, // 400K TPM with 10% buffer to stay under the limit
}

// OpenAIRateLimit defines the default rate limits for OpenAI API
var OpenAIRateLimit = RateLimit{
	RPM: 10 * 1000,           // 10K RPM
	TPM: 10 * 1_000_000 * .9, // 10M TPM with 10% buffer to stay under the limit
}
