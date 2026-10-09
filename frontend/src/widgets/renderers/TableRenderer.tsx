import type { WidgetItem } from '../../api/client'
import { ItemLink } from './ItemLink'
import type { ItemsRendererProps } from './types'

type Column = { key: keyof Pick<WidgetItem, 'title' | 'date' | 'label' | 'value'>; heading: string }

const COLUMNS: Column[] = [
  { key: 'title', heading: 'Name' },
  { key: 'date', heading: 'Date' },
  { key: 'label', heading: 'Detail' },
  { key: 'value', heading: 'Value' },
]

/** `table`: columns title | date | label | value, showing only columns that have any data. */
export function TableRenderer({ items }: ItemsRendererProps) {
  const columns = COLUMNS.filter((col) => items.some((item) => item[col.key] !== ''))
  // Raw search results (the fallback when shaping fails) have no label or
  // value; show their snippets under the name so that content isn't lost.
  const showSnippets = !items.some((item) => item.label || item.value)

  return (
    // Narrow widgets scroll the table sideways instead of squashing it.
    <div className="-mx-1 overflow-x-auto px-1">
      <table className="w-full border-collapse text-left text-sm">
        <thead>
          <tr className="border-b border-line">
            {columns.map((col) => (
              <th key={col.key} scope="col" className="pb-2 pr-4 text-xs font-medium text-ink-muted last:pr-0">
                {col.heading}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-line">
          {items.map((item, i) => (
            <tr key={i}>
              {columns.map((col) => (
                <td
                  key={col.key}
                  className={`py-2.5 pr-4 align-top last:pr-0 ${
                    col.key === 'title' ? 'font-medium text-ink' : 'text-ink-soft'
                  } ${col.key === 'value' ? 'tabular-nums' : ''}`}
                >
                  {col.key === 'title' ? (
                    <>
                      <ItemLink url={item.url}>{item.title}</ItemLink>
                      {showSnippets && item.snippet && (
                        <p className="mt-1 line-clamp-2 font-normal leading-relaxed text-ink-soft">{item.snippet}</p>
                      )}
                    </>
                  ) : (
                    item[col.key]
                  )}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
