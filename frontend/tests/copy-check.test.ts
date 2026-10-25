// npm test: the copy check's rules (scripts/copy-check.mjs).
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { problemWith, textsInGo, textsInTs } from '../scripts/copy-check.mjs'

const problems = (texts: { text: string }[]) => texts.map((t) => problemWith(t.text)).filter(Boolean)

test('JSX text that says widget is flagged', () => {
  const found = textsInTs('const x = (\n  <p>\n    Add a widget\n  </p>\n)')
  assert.deepEqual(problems(found), ['says "widget" (say "card")'])
  assert.equal(found.find((t) => t.text.includes('widget'))?.line, 2)
})

test('a string literal that says widget is flagged', () => {
  assert.equal(problems(textsInTs("const label = 'Remove this widget'")).length, 1)
  assert.equal(problems(textsInTs('const t = `Building your widget…`')).length, 1)
})

test('keys, paths and comments are not copy', () => {
  const code = [
    "import { useNow } from './widgets/updatedAgo'",
    "queryKey: ['widgets', 'custom', id],",
    '// Add a widget: the old wording, in a comment',
    '/** The add-a-widget tile. */',
    "const url = 'https://example.com/a//b'",
  ].join('\n')
  assert.deepEqual(problems(textsInTs(code)), [])
})

test('interpolations and code between tags are not copy', () => {
  const code = [
    'const label = `Refresh ${widget.title}`',
    '<p>{note}</p>',
    ") : widget?.source === 'weather_forecast' ? (",
    '<WeatherCredit />',
  ].join('\n')
  assert.deepEqual(problems(textsInTs(code)), [])
})

test('Go messages that say widget are flagged; other Go strings are not', () => {
  const code = [
    'respond.Error(w, http.StatusNotFound, "That widget doesn\'t exist.", "")',
    'return &UpstreamError{Reason: "declined", Hint: "Try describing the widget differently."}',
    'logger.Info("widget saved", "widget_id", id)',
    '// respond.Error(w, 404, "an old widget message", "")',
  ].join('\n')
  const found = textsInGo(code)
  assert.equal(problems(found).length, 2)
  assert.deepEqual(
    found.filter((t) => problemWith(t.text)).map((t) => t.line),
    [1, 2],
  )
})

test('banned phrases', () => {
  assert.equal(problemWith('Your page, powered by AI'), 'says "powered by ai"')
  assert.equal(problemWith('Keep the family in sync'), 'says "in sync"')
  assert.equal(problemWith('Morning, Maya. Soccer at 4.'), null)
})
