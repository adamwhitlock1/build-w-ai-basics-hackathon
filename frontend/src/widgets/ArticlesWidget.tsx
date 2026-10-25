import { useArticles, usePageOwner, useRefreshCuratedWidget } from '../api/hooks'
import { CuratedWidgetCard } from './CuratedWidgetCard'

/**
 * The Articles widget: a few news articles picked from the interests of
 * whoever's page it's on, with a refresh button. It loads its own data, so it
 * can sit anywhere on a page.
 */
export function ArticlesWidget() {
  const articles = useArticles()
  const refresh = useRefreshCuratedWidget('articles')
  // On a family member's page, a parent reads it, so it speaks about them.
  const { isYou } = usePageOwner()
  return (
    <CuratedWidgetCard
      result={articles}
      onRefresh={refresh}
      title={isYou ? 'Articles for you' : 'Their articles'}
      noun="articles"
      loadingMessage={isYou ? 'Finding articles for you…' : 'Finding articles for them…'}
      missing={{
        no_interests: isYou
          ? {
              message: 'Tell Kinlight what you’re into and articles will show up here.',
              action: 'Add your interests',
              field: 'interests',
            }
          : {
              message: 'Tell Kinlight what they’re into and articles will show up here.',
              action: 'Add their interests',
              field: 'interests',
            },
      }}
    />
  )
}
