import { useMe, useSetTheme } from '../api/hooks'
import type { Theme } from '../api/client'
import { themeNames } from '../theme'

const THEMES: Theme[] = ['auto', 'light', 'dark']

/**
 * The member's theme: Automatic, Daybreak or Porch light, as three radio
 * buttons drawn as one segmented control (arrow keys move between them).
 * The page switches at once; if saving fails it switches back and says so.
 */
export function ThemeSwitch() {
  const me = useMe().data
  const setTheme = useSetTheme()
  if (!me) return null

  return (
    <fieldset className="min-w-0">
      <legend className="mb-1.5 px-2 text-xs font-medium text-ink-muted">Theme</legend>
      <div className="flex rounded-2xl bg-bg p-1 ring-1 ring-line">
        {THEMES.map((theme) => (
          <label
            key={theme}
            className="flex-1 cursor-pointer rounded-xl px-2 py-1.5 text-center text-xs font-medium whitespace-nowrap text-ink-soft transition-colors hover:text-ink has-checked:bg-surface has-checked:text-ink has-checked:shadow-card has-focus-visible:outline-2 has-focus-visible:outline-offset-1 has-focus-visible:outline-primary"
          >
            <input
              type="radio"
              name="theme"
              value={theme}
              checked={me.user.theme === theme}
              onChange={() => setTheme.mutate(theme)}
              className="sr-only"
            />
            {themeNames[theme]}
          </label>
        ))}
      </div>
      {setTheme.isError && (
        <p role="alert" className="mt-1.5 px-2 text-xs text-danger">
          Your theme wasn’t saved. {setTheme.error.hint || setTheme.error.message}
        </p>
      )}
    </fieldset>
  )
}
