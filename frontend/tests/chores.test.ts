import test from 'node:test'
import assert from 'node:assert/strict'
import { choreKey, midnightDelay, completionBody, isStaleDayError } from '../src/chores/model.ts'

test('household timer uses server instants across DST and subtracts elapsed time', () => {
  const day = {
    date: '2026-03-08',
    timeZone: 'America/Denver',
    serverNow: '2026-03-08T00:00:00-07:00',
    nextResetAt: '2026-03-09T00:00:00-06:00',
  }
  assert.equal(midnightDelay(day), 23 * 3600_000)
  assert.equal(midnightDelay(day, 23 * 3600_000 + 1), 0)
  assert.equal(
    midnightDelay({
      ...day,
      serverNow: '2026-11-01T00:00:00-06:00',
      nextResetAt: '2026-11-02T00:00:00-07:00',
    }),
    25 * 3600_000,
  )
})
test('keys isolate viewers, subjects, and summaries', () => {
  assert.notDeepEqual(choreKey(1, 3), choreKey(2, 3))
  assert.notDeepEqual(choreKey(1, 3), choreKey(1, 4))
  assert.notDeepEqual(choreKey(1, 1), choreKey(1, 'family'))
})
test('completion is an explicit state and stale day is distinct from the count limit', () => {
  assert.deepEqual(completionBody('2026-10-25', false), { date: '2026-10-25', completed: false })
  assert.equal(
    isStaleDayError({ status: 409, message: 'A new chore day has started. Reload today and check again.' }),
    true,
  )
  assert.equal(
    isStaleDayError({ status: 409, message: 'Each member can have up to 30 active chores.' }),
    false,
  )
})
