// Small helpers for showing family members and invites: names, initials
// avatars, role names, invite links and their dates.

/**
 * What to call someone in a heading, until people have names of their own:
 * the part of their email before the "@", capitalised ("maya@example.com" →
 * "Maya", "kid.whitlock@example.com" → "Kid.whitlock").
 */
export function nameFor(email: string): string {
  const local = email.split('@')[0] ?? ''
  // Array.from splits by character, so an accented letter or emoji stays whole.
  const [first = '', ...rest] = Array.from(local)
  return first.toUpperCase() + rest.join('')
}

/**
 * Two letters for an avatar, taken from the part of an email before the "@":
 * the first letter of each of the first two words ("sam.rivera" → "SR"), or
 * the first two letters of a single word ("maya" → "MA", "kid1" → "KI").
 */
export function initialsFor(email: string): string {
  const local = email.split('@')[0] ?? ''
  // Words are runs of letters or digits; dots, dashes, "+" and "_" separate them.
  const words = local.split(/[^\p{L}\p{N}]+/u).filter(Boolean)
  if (words.length === 0) return '?'
  // Array.from splits by character, so an accented letter or emoji stays whole.
  const firstChars = (word: string) => Array.from(word)
  if (words.length >= 2) {
    return (firstChars(words[0])[0] + firstChars(words[1])[0]).toUpperCase()
  }
  // One word: prefer its letters ("kid1" → "KI"), unless it has none ("1234").
  const letters = firstChars(words[0]).filter((ch) => /\p{L}/u.test(ch))
  const chars = letters.length > 0 ? letters : firstChars(words[0])
  return chars.slice(0, 2).join('').toUpperCase()
}

/**
 * Soft background tints for avatars. Dark ink reads well on each. Spelled out
 * in full so Tailwind can find every class.
 */
const AVATAR_TINTS = [
  'bg-amber-100',
  'bg-indigo-100',
  'bg-rose-100',
  'bg-emerald-100',
  'bg-sky-100',
  'bg-violet-100',
]

/** A tint for someone's avatar. The same email always gets the same one. */
export function avatarTint(email: string): string {
  let hash = 0
  for (const ch of email) {
    // `>>> 0` keeps the number a whole, positive 32-bit value as it grows.
    hash = (hash * 31 + (ch.codePointAt(0) ?? 0)) >>> 0
  }
  return AVATAR_TINTS[hash % AVATAR_TINTS.length]
}

/** What the app calls each role: parents (admins) can invite and manage; everyone else is a kid. */
export function roleName(isAdmin: boolean): 'Parent' | 'Kid' {
  return isAdmin ? 'Parent' : 'Kid'
}

/** The full invite link to share, from the path the server gives ("/invite/<token>"). */
export function inviteLink(path: string): string {
  return `${window.location.origin}${path}`
}

const shortDate = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' })

/** "Oct 25" for an RFC 3339 timestamp, or "" if it can't be read. */
export function formatShortDate(timestamp: string): string {
  const date = new Date(timestamp)
  return Number.isNaN(date.getTime()) ? '' : shortDate.format(date)
}

const dateTime = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })

/** "Oct 24, 5:58 PM" for an RFC 3339 timestamp, or "" if it can't be read. */
export function formatWhen(timestamp: string): string {
  const date = new Date(timestamp)
  return Number.isNaN(date.getTime()) ? '' : dateTime.format(date)
}

/**
 * How to name a family in a sentence: "the Rivera family", but just "The
 * Riveras" (not "the The Riveras family") and "the Rivera Family" (not "the
 * Rivera Family family").
 */
export function familyPhrase(name: string): string {
  const trimmed = name.trim()
  if (/^the\s/i.test(trimmed)) return trimmed
  if (/\sfamily$/i.test(trimmed)) return `the ${trimmed}`
  return `the ${trimmed} family`
}
