import { useArticles } from '../api/hooks'
import { CuratedWidgetCard } from './CuratedWidgetCard'

/**
 * The Articles widget: a few news articles picked from your interests, with
 * a refresh button. It loads its own data, so it can sit anywhere on a page.
 */
export function ArticlesWidget() {
  const articles = useArticles()
  return (
    <CuratedWidgetCard
      result={articles}
      title="Articles for you"
      noun="articles"
      loadingMessage="Finding articles for you…"
      missing={{
        no_interests: {
          message: 'Tell Porchlight what you’re into and articles will show up here.',
          action: 'Add your interests',
          field: 'interests',
        },
      }}
    />
  )
}
