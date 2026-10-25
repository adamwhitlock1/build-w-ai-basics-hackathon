import { useEffect, useId, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import type { SavedWidget } from '../api/client'
import { useCreateWidgetFromPrompt } from '../api/hooks'
import { CloseIcon, PlusIcon } from './icons'

const EXAMPLES = [
  'upcoming SpaceX launches',
  'show me local farmers markets this weekend',
  'give me a quote of the day',
  'this week’s weather',
]
const EXAMPLE_INTERVAL_MS = 3500
const MAX_PROMPT_LENGTH = 500

interface DescribeWidgetPromptProps {
  /** DOM id of the prompt, so the "+" button can point at it with aria-controls. */
  id: string
  /** Called with the new widget once the server has built and saved it. */
  onCreated: (widget: SavedWidget) => void
  /** Called when the person dismisses the prompt (close button or Escape). */
  onClose: () => void
}

/**
 * The kernel moment: describe a widget in plain words and get a live one back.
 * Inline on the page (no modal), so the rest of the page stays usable.
 */
export function DescribeWidgetPrompt({ id, onCreated, onClose }: DescribeWidgetPromptProps) {
  const [prompt, setPrompt] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const headingId = useId()
  const example = useCyclingExample(EXAMPLES, EXAMPLE_INTERVAL_MS)
  // The insert lives in the hook-level callback, so it runs even if this
  // prompt is gone by the time the server answers.
  const createWidget = useCreateWidgetFromPrompt({ onCreated })
  const { status } = createWidget

  // Focus the input when the prompt opens, and again when it comes back after
  // a failed attempt (the thinking state replaces it while waiting).
  useEffect(() => {
    if (status === 'idle' || status === 'error') {
      inputRef.current?.focus()
    }
  }, [status])

  function submit(text: string) {
    const trimmed = text.trim()
    if (!trimmed || createWidget.isPending) return
    createWidget.mutate(trimmed, {
      // UI-local only; adding the widget is the hook's job (see above).
      onSuccess: () => setPrompt(''),
    })
  }

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    submit(prompt)
  }

  function handleChange(value: string) {
    setPrompt(value)
    // The error describes the prompt that failed; once it's edited, clear it.
    if (createWidget.isError) createWidget.reset()
  }

  function handleKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Escape') onClose()
  }

  return (
    <section
      id={id}
      aria-labelledby={headingId}
      className="rounded-3xl border border-line bg-surface p-5 shadow-card sm:p-6"
    >
      <div className="mb-3 flex items-center justify-between gap-4">
        <h2 id={headingId} className="text-xl text-ink">
          Add a widget
        </h2>
        <div className="flex items-center gap-3">
          <p className="hidden text-xs text-ink-muted sm:block">Describe it in your own words</p>
          {/* Hidden, not removed, while thinking: no controls then, and no layout jump. */}
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className={`rounded-full p-1.5 text-ink-muted transition-colors hover:bg-bg hover:text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary ${
              createWidget.isPending ? 'invisible' : ''
            }`}
          >
            <CloseIcon />
          </button>
        </div>
      </div>

      {createWidget.isPending ? (
        <ThinkingState prompt={createWidget.variables ?? prompt} />
      ) : (
        <form onSubmit={handleSubmit} className="flex items-center gap-2 sm:gap-3">
          <input
            ref={inputRef}
            type="text"
            value={prompt}
            onChange={(e) => handleChange(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={`try: ${example}`}
            maxLength={MAX_PROMPT_LENGTH}
            aria-label="Describe a widget"
            aria-describedby={createWidget.isError ? `${headingId}-error` : undefined}
            autoComplete="off"
            enterKeyHint="go"
            className="h-14 min-w-0 flex-1 rounded-2xl border border-control bg-bg px-4 text-base text-ink placeholder:text-ink-muted focus:border-primary/50 focus:bg-surface focus:outline-none focus:ring-4 focus:ring-primary/10"
          />
          <button
            type="submit"
            disabled={!prompt.trim()}
            className="flex h-14 shrink-0 items-center gap-2 rounded-2xl bg-primary px-4 font-medium text-on-primary transition-colors hover:bg-primary-strong focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-primary sm:px-5"
          >
            <PlusIcon />
            <span className="sr-only sm:not-sr-only">Add</span>
          </button>
        </form>
      )}

      {createWidget.isError && (
        <div
          id={`${headingId}-error`}
          role="alert"
          className="mt-3 flex flex-col gap-3 rounded-2xl bg-danger-soft px-4 py-3 text-sm sm:flex-row sm:items-center sm:justify-between"
        >
          <div>
            <p className="font-medium text-danger">{createWidget.error.message}</p>
            {createWidget.error.hint && <p className="mt-0.5 text-ink-soft">{createWidget.error.hint}</p>}
          </div>
          <button
            type="button"
            onClick={() => submit(createWidget.variables ?? prompt)}
            className="shrink-0 self-start rounded-xl border border-primary/30 bg-surface px-3.5 py-2 font-medium text-primary transition-colors hover:bg-primary-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary sm:self-auto"
          >
            Try again
          </button>
        </div>
      )}
    </section>
  )
}

/**
 * Replaces the input while the widget is being built: calm, with no controls.
 * It takes focus when it appears, so focus stays in the prompt (rather than
 * falling back to the page) and screen readers read out what's happening.
 */
function ThinkingState({ prompt }: { prompt: string }) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    ref.current?.focus()
  }, [])

  return (
    <div
      ref={ref}
      tabIndex={-1}
      className="flex h-14 items-center gap-3 rounded-2xl bg-accent-soft/60 px-4 focus:outline-none"
    >
      <span
        aria-hidden="true"
        className="size-5 shrink-0 rounded-full border-2 border-accent/25 border-t-accent motion-safe:animate-spin-calm"
      />
      <p className="min-w-0 truncate text-ink">
        Building your widget…
        <span className="ml-2 text-ink-muted">“{prompt}”</span>
      </p>
    </div>
  )
}

/** Rotates through example prompts to hint at what's possible. */
function useCyclingExample(examples: string[], intervalMs: number): string {
  const [index, setIndex] = useState(0)
  useEffect(() => {
    const id = window.setInterval(() => setIndex((i) => (i + 1) % examples.length), intervalMs)
    return () => window.clearInterval(id)
  }, [examples.length, intervalMs])
  return examples[index]
}
