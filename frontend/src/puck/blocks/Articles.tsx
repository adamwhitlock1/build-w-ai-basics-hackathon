import { ArticlesWidget } from '../../widgets/ArticlesWidget'

/**
 * The Articles card, as placed on the page and in the Arrange editor. It reads
 * its picks through the same query (and query key) wherever it's shown, so
 * switching to Arrange doesn't fetch them again.
 */
export function ArticlesBlock() {
  return <ArticlesWidget />
}
