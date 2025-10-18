package turbo_run

type RateLimit struct {
	RPM int
	TPM int
}

var GroqRateLimit = RateLimit{
	RPM: 1 * 1000,           // 1K RPM
	TPM: (400 * 1000) * .85, // 400K TPM with 15% buffer to stay under the limit
}

var OpenAIRateLimit = RateLimit{
	RPM: 10 * 1000,           // 10K RPM
	TPM: 10 * 1_000_000 * .9, // 10M TPM with 10% buffer to stay under the limit
}
