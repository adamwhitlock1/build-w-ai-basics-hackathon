package aikit

import "time"

// Files named export_test.go are compiled only by "go test", so these never
// ship, yet the external aikit_test package can still use them.

// Upstream is upstream, for the error-mapping tests.
var Upstream = upstream

// UsageModel is usageModel.
var UsageModel = usageModel

// SetRetryDelay shortens the rate-limit pause so tests run fast.
func SetRetryDelay(k *Kit, d time.Duration) { k.retryDelay = d }

// KidSystemPrompt is kidSystemPrompt.
var KidSystemPrompt = kidSystemPrompt

// The prompt blocks, for checking which ones a system prompt contains.
const (
	PersonaYoungerPrompt = personaYoungerPrompt
	PersonaTeenPrompt    = personaTeenPrompt
	ModeHomeworkPrompt   = modeHomeworkPrompt
	ModeExplorePrompt    = modeExplorePrompt
	HomeworkRule         = homeworkRule
	CrisisPrompt         = crisisPrompt
	InjectionRule        = injectionRule
	SafetyTitleRequest   = safetyTitleRequest
)
