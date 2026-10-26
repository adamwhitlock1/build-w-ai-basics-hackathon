export interface ChoreDay {
  date: string
  timeZone: string
  serverNow: string
  nextResetAt: string
}
export interface Checklist extends ChoreDay {
  items: { id: number; title: string; completed: boolean }[]
  completedCount: number
  totalCount: number
}
export interface ChoreSummary extends ChoreDay {
  members: { userId: number; email: string; completedCount: number; totalCount: number }[]
}
export const choreKey = (viewer: number, subject: number | 'family') => ['chores', viewer, subject] as const
export function midnightDelay(day: ChoreDay, elapsed = 0) {
  return Math.max(0, Date.parse(day.nextResetAt) - Date.parse(day.serverNow) - elapsed)
}
export function completionBody(date: string, completed: boolean) {
  return { date, completed }
}
export function isStaleDayError(error: { status?: number; message?: string }) {
  return error.status === 409 && error.message?.startsWith('A new chore day') === true
}
/** The checklist's chores split into those still to do and those done, each keeping its order. */
export function splitByDone<T extends { completed: boolean }>(items: T[]) {
  return { open: items.filter((c) => !c.completed), done: items.filter((c) => c.completed) }
}
