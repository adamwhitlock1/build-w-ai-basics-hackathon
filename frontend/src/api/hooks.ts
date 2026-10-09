import { useMutation } from '@tanstack/react-query'
import { apiFetch, type ApiError, type Widget } from './client'

/** Identifies widget-creation mutations, e.g. for `useIsMutating`. */
export const createWidgetKey = ['widgets', 'from-prompt'] as const

interface CreateWidgetOptions {
  /**
   * Called with the new widget. Unlike a callback passed to `mutate`, this one
   * still runs if the calling component unmounts before the server answers,
   * so a widget the server built is never dropped.
   */
  onCreated?: (widget: Widget) => void
}

/**
 * Turns a plain-language prompt into a live widget.
 * Call `mutate(prompt)`; the server trims and validates it (1–500 chars).
 */
export function useCreateWidgetFromPrompt({ onCreated }: CreateWidgetOptions = {}) {
  // apiFetch normalises every failure into an ApiError, so the error type is exact.
  return useMutation<Widget, ApiError, string>({
    mutationKey: createWidgetKey,
    mutationFn: (prompt) =>
      apiFetch<Widget>('/api/widgets/from-prompt', {
        method: 'POST',
        body: { prompt },
      }),
    onSuccess: (widget) => onCreated?.(widget),
  })
}
