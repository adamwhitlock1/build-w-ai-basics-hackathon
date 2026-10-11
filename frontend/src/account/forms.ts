// Small helpers shared by the login, sign-up, create-family and interests
// forms. The checks mirror the server's so mistakes are caught before a round
// trip, but the server always has the final say.

/** The server's minimum password length. */
export const MIN_PASSWORD_LENGTH = 8

/** The server's limit for interests and for location, each. */
export const PROFILE_MAX_LENGTH = 500

/** A loose "looks like an email" check, like the server's: something, one "@", then a dotted domain. */
export function looksLikeEmail(value: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)
}

/**
 * Turns a typed list of email addresses into tidy ones: trimmed, lowercased,
 * with blanks and repeats dropped. Commas separate them; semicolons, spaces
 * and line breaks work too, since none can appear inside an address.
 */
export function splitEmails(text: string): string[] {
  const emails = text
    .split(/[\s,;]+/)
    .map((email) => email.toLowerCase())
    .filter(Boolean)
  return [...new Set(emails)]
}

/** Moves focus to a form's control by its `name`, optionally selecting its text. */
export function focusField(form: HTMLFormElement, name: string, { select = false } = {}) {
  const control = form.elements.namedItem(name)
  if (control instanceof HTMLInputElement || control instanceof HTMLTextAreaElement) {
    control.focus()
    if (select) control.select()
  }
}
