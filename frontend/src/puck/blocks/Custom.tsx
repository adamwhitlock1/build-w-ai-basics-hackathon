import { useHomepage } from '../../api/hooks'
import { WidgetCard } from '../../widgets/WidgetCard'

interface CustomBlockProps {
  /** Which saved widget to show (SavedWidget.id). */
  widgetId: number
  /** Just built: amber border and a soft fade-in (see WidgetCard). */
  fresh?: boolean
}

/**
 * A widget you described, as placed on the page and in the Arrange editor.
 * Its content came with the page (Homepage.customWidgets), so it's read from
 * that cached copy rather than fetched on its own.
 */
export function CustomBlock({ widgetId, fresh = false }: CustomBlockProps) {
  const { data: page } = useHomepage()
  const widget = page.customWidgets.find((w) => w.id === widgetId)

  if (!widget) {
    // The layout mentions a widget this page doesn't have (the server tidies
    // that up on the next load), so say so quietly rather than show nothing.
    return (
      <div className="flex min-h-40 items-center justify-center rounded-3xl border-2 border-dashed border-line p-6 text-center text-sm text-ink-muted">
        This widget is gone
      </div>
    )
  }
  return <WidgetCard widget={widget} fresh={fresh} />
}
