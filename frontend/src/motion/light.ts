// The lantern's rules, as plain functions (brand/lantern-ux.md). They have
// no imports, so tests/light.test.ts runs them with Node's own test runner.

/** Why something wants the one full-strength glow, highest priority first. */
export const LIGHT_KINDS = ['thinking', 'arrival', 'briefing'] as const
export type LightKind = (typeof LIGHT_KINDS)[number]

export interface LightClaim {
  id: string
  kind: LightKind
  /** When the claim was made (ms); within a kind, the newest wins. */
  since: number
}

/**
 * Which claim gets the one full-strength glow on the screen: the AI while it
 * writes, otherwise a card that just arrived, otherwise the morning briefing.
 * Within a kind, the newest claim wins. Undefined when nothing claims it.
 */
export function holder(claims: readonly LightClaim[]): string | undefined {
  let best: LightClaim | undefined
  for (const claim of claims) {
    if (
      !best ||
      LIGHT_KINDS.indexOf(claim.kind) < LIGHT_KINDS.indexOf(best.kind) ||
      (claim.kind === best.kind && claim.since > best.since)
    ) {
      best = claim
    }
  }
  return best?.id
}

/** How much earlier than its card's mount a result may be and still count as written during the visit. */
export const ARRIVAL_SLACK_MS = 5_000

/**
 * Whether a card's content just arrived: its update time changed to one
 * written during this visit (no earlier than the card's mount, less a little
 * slack). A remembered result loaded as the page opens isn't an arrival.
 */
export function arrived(prev: string | undefined, next: string | undefined, mountedAt: number): boolean {
  if (!next || next === prev) return false
  const at = Date.parse(next)
  return !Number.isNaN(at) && at >= mountedAt - ARRIVAL_SLACK_MS
}

/** The freshness edge's light just after an update: below the full glow's 1. */
export const EDGE_MAX = 0.45
/** How long a card's edge takes to settle to no light. */
export const EDGE_SETTLE_MS = 6 * 60 * 60 * 1000

/** A card's freshness edge, from EDGE_MAX just after an update down to 0 after six hours. */
export function edgeLevel(updatedAt: string | undefined, now: number): number {
  if (!updatedAt) return 0
  const at = Date.parse(updatedAt)
  if (Number.isNaN(at)) return 0
  const age = Math.max(0, now - at)
  return age >= EDGE_SETTLE_MS ? 0 : EDGE_MAX * (1 - age / EDGE_SETTLE_MS)
}

/** Whether it's morning where the viewer is: before noon, local time. */
export function isMorning(now: Date): boolean {
  return now.getHours() < 12
}
