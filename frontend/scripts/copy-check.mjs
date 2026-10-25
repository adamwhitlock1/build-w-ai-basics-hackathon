// Checks the words people read against the voice in brand/brief.md:
// cards are "cards", never "widgets", and a few phrases are never used.
//
//   npm run check:copy
//
// It reads the frontend's string literals and JSX text, and the backend's
// user-facing messages (string literals on lines that call respond.Error or
// set a Hint). Text with a space in it is copy; a lone word like 'widgets'
// is a key or a path, so code can keep saying widget. Comments are skipped.
// The AI's instructions in backend/internal/llm aren't copy and aren't read.

import { readFileSync, readdirSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

/** Phrases the brief rules out (section 8). */
export const BANNED = ['mental load', 'in sync', 'supercharge', 'seamless', 'powered by ai']

const WIDGET = /\bwidgets?\b/i

/** Why a piece of visible text breaks the voice, or null if it doesn't. */
export function problemWith(raw) {
  // An interpolation like ${widget.title} is code, not words.
  const text = raw.replace(/\$\{[^}]*\}/g, '')
  const lower = text.toLowerCase()
  const phrase = BANNED.find((p) => lower.includes(p))
  if (phrase) return `says "${phrase}"`
  if (/\s/.test(text.trim()) && WIDGET.test(text)) return 'says "widget" (say "card")'
  return null
}

/** Code with its comments blanked out, keeping line numbers. */
function withoutComments(code) {
  return code
    .replace(/\/\*[\s\S]*?\*\//g, (c) => c.replace(/[^\n]/g, ' '))
    .replace(/(^|[^:"'`])\/\/.*$/gm, (_, before) => before)
}

/** The string literals and JSX text in TypeScript/TSX source, with line numbers. */
export function textsInTs(code) {
  const clean = withoutComments(code)
  const found = []
  const add = (index, text) => found.push({ line: clean.slice(0, index).split('\n').length, text })
  for (const m of clean.matchAll(/'((?:[^'\\\n]|\\.)*)'|"((?:[^"\\\n]|\\.)*)"|`((?:[^`\\]|\\.)*)`/g)) {
    add(m.index, m[1] ?? m[2] ?? m[3])
  }
  // JSX text: what sits between a tag's > and the next <, outside braces.
  // A stretch with code operators in it is code between two tags, not text.
  for (const m of clean.matchAll(/>([^<>{}]*[A-Za-z][^<>{}]*)</g)) {
    if (!/===|\?\.|=>|&&|\|\|/.test(m[1])) add(m.index, m[1])
  }
  return found
}

/** The string literals in Go source lines that write a message people read. */
export function textsInGo(code) {
  const found = []
  code.split('\n').forEach((line, i) => {
    if (!/respond\.Error|Hint:/.test(line) || /^\s*\/\//.test(line)) return
    for (const m of line.matchAll(/"((?:[^"\\]|\\.)*)"|`([^`]*)`/g)) found.push({ line: i + 1, text: m[1] ?? m[2] })
  })
  return found
}

function filesUnder(dir, keep) {
  const out = []
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) out.push(...filesUnder(path, keep))
    else if (keep(entry.name)) out.push(path)
  }
  return out
}

/** Every problem in the frontend and backend, as "file:line: text (why)". */
export function check(root) {
  const problems = []
  const scan = (files, extract) => {
    for (const file of files) {
      for (const { line, text } of extract(readFileSync(file, 'utf8'))) {
        const why = problemWith(text)
        if (why) problems.push(`${relative(root, file)}:${line}: ${text.trim()} (${why})`)
      }
    }
  }
  scan(
    filesUnder(join(root, 'frontend/src'), (n) => /\.tsx?$/.test(n) && n !== 'routeTree.gen.ts'),
    textsInTs,
  )
  scan(
    filesUnder(join(root, 'backend'), (n) => n.endsWith('.go') && !n.endsWith('_test.go')),
    textsInGo,
  )
  return problems
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const root = fileURLToPath(new URL('../..', import.meta.url))
  const problems = check(root)
  if (problems.length > 0) {
    console.error(problems.join('\n'))
    console.error(`${problems.length} piece(s) of copy break the voice in brand/brief.md.`)
    process.exit(1)
  }
  console.log('Copy: no "widget" and no banned phrases in what people read.')
}
