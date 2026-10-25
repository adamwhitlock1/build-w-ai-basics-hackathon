import { test } from 'node:test'
import assert from 'node:assert/strict'
import { demoPaused, isDemoLimit } from '../src/public/demoState.ts'

test('daily exhaustion clears at the server reset instant, including DST days', () => {
  for (const reset of ['2026-03-09T00:00:00-06:00', '2026-11-02T00:00:00-07:00']) {
    const quota = { modelsRemaining: 0, searchesRemaining: 10, reset }
    const boundary = Date.parse(reset)
    assert.equal(demoPaused(quota, boundary - 1), true)
    assert.equal(demoPaused(quota, boundary), false)
  }
  assert.equal(demoPaused(undefined), false)
})
test('search exhaustion pauses refresh while provider failures remain distinct', () => {
  assert.equal(demoPaused({ modelsRemaining: 5, searchesRemaining: 0, reset: '2030-01-01T00:00:00Z' }), true)
  assert.equal(isDemoLimit('Demo allowance reached. Live AI is paused until tomorrow.'), true)
  assert.equal(isDemoLimit('OpenRouter is out of credits'), false)
  assert.equal(isDemoLimit('Could not reach the server'), false)
})
