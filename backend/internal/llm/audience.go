package llm

import "porchlight/internal/sources"

// kidGuidance is added to every system prompt when the content is for a kid.
// The search provider's strict safe-search is the first filter; this asks
// the model to be the second.
const kidGuidance = `This member is a child or teenager, under 18. Everything you choose must be suitable for them:
- Leave out sexual content, graphic violence, drugs, alcohol, gambling, hateful content, and adults-only (18+ or 21+) events and venues such as bars, nightclubs, and casinos.
- Steer search queries toward age-appropriate angles of what they asked for.
- If you are unsure whether something is suitable, leave it out.`

// forAudience returns system with kidGuidance appended when aud is
// sources.Kids, and unchanged otherwise.
func forAudience(system string, aud sources.Audience) string {
	if aud != sources.Kids {
		return system
	}
	return system + "\n\n" + kidGuidance
}
