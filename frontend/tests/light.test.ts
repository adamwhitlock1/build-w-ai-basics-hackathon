// npm test: Node's test runner on src/motion/light.ts, with TypeScript's
// types stripped by Node itself. Kept outside src, so the app's tsconfig
// (which has no Node types) never builds it.
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { EDGE_MAX, arrived, edgeLevel, holder, isMorning, type LightClaim } from '../src/motion/light.ts'

test('holder: thinking, then an arrival, then the briefing', () => {
  const briefing: LightClaim = { id: 'briefing', kind: 'briefing', since: 1 }
  const card: LightClaim = { id: 'card', kind: 'arrival', since: 2 }
  const reply: LightClaim = { id: 'reply', kind: 'thinking', since: 3 }
  assert.equal(holder([]), undefined)
  assert.equal(holder([briefing]), 'briefing')
  assert.equal(holder([briefing, card]), 'card')
  assert.equal(holder([card, reply, briefing]), 'reply')
})

test('holder: the newest claim within a kind', () => {
  const older: LightClaim = { id: 'older', kind: 'arrival', since: 10 }
  const newer: LightClaim = { id: 'newer', kind: 'arrival', since: 20 }
  assert.equal(holder([older, newer]), 'newer')
  assert.equal(holder([newer, older]), 'newer')
})

test('arrived: only content written during the visit', () => {
  const mountedAt = Date.parse('2026-10-25T08:00:00Z')
  const cached = '2026-10-25T06:00:00Z'
  const written = '2026-10-25T08:00:30Z'
  // A remembered result loaded as the page opened.
  assert.equal(arrived(undefined, cached, mountedAt), false)
  // Nothing changed, or nothing yet.
  assert.equal(arrived(written, written, mountedAt), false)
  assert.equal(arrived(cached, undefined, mountedAt), false)
  // A refresh, or a first result written during the visit.
  assert.equal(arrived(cached, written, mountedAt), true)
  assert.equal(arrived(undefined, written, mountedAt), true)
  // Written just before the card mounted (the page was still loading).
  assert.equal(arrived(undefined, '2026-10-25T07:59:57Z', mountedAt), true)
  assert.equal(arrived(undefined, 'not a time', mountedAt), false)
})

test('edgeLevel: from full just after an update to nothing after six hours', () => {
  const at = Date.parse('2026-10-25T08:00:00Z')
  const updated = '2026-10-25T08:00:00Z'
  const hour = 60 * 60 * 1000
  assert.equal(edgeLevel(updated, at), EDGE_MAX)
  assert.ok(Math.abs(edgeLevel(updated, at + 3 * hour) - EDGE_MAX / 2) < 1e-9)
  assert.equal(edgeLevel(updated, at + 6 * hour), 0)
  assert.equal(edgeLevel(updated, at + 9 * hour), 0)
  // A clock a little behind the server's never lights more than full.
  assert.equal(edgeLevel(updated, at - hour), EDGE_MAX)
  assert.equal(edgeLevel(undefined, at), 0)
})

test('isMorning: until local noon', () => {
  assert.equal(isMorning(new Date(2026, 9, 25, 7, 30)), true)
  assert.equal(isMorning(new Date(2026, 9, 25, 11, 59)), true)
  assert.equal(isMorning(new Date(2026, 9, 25, 12, 0)), false)
  assert.equal(isMorning(new Date(2026, 9, 25, 19, 0)), false)
})
