import { Link } from '@tanstack/react-router'
import { BrandMark } from '../components/BrandMark'
import { PorchScene } from '../components/PorchScene'
import { usePublicConfig } from './config'

const action = 'inline-flex items-center justify-center rounded-full bg-primary px-6 py-3 font-medium text-on-primary hover:bg-primary-strong focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-primary'
export function LandingPage() {
  const { data, isError } = usePublicConfig()
  return <div className="min-h-dvh bg-bg text-ink">
    <header className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4 px-6 py-6 sm:px-10">
      <BrandMark />
      <Link to="/login" className="rounded px-2 py-2 text-primary underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-primary">Family sign in</Link>
    </header>
    <main>
      <section className="mx-auto grid max-w-7xl items-center gap-10 px-6 py-12 sm:px-10 lg:grid-cols-2 lg:py-20">
        <div>
          <p className="text-sm font-semibold uppercase tracking-widest text-ink-muted">A little light for family life</p>
          <h1 className="mt-5 max-w-xl text-5xl leading-tight sm:text-6xl">Everyone’s day.<br />A place of their own.</h1>
          <p className="mt-6 max-w-lg text-lg leading-relaxed text-ink-soft">Kinlight brings the day’s plans, daily chores, and things you’re curious about into a homepage that feels like yours.</p>
          <div className="mt-8 flex flex-wrap items-center gap-4">
            {data?.demoUrl ? <a href={data.demoUrl + (data.demoUrl.endsWith('/') ? 'login' : '/login')} className={action}>Try the family demo</a> : <p className="text-sm text-ink-muted">{isError ? 'Demo details are unavailable right now.' : 'The hosted demo link is coming soon.'}</p>}
            {data?.videoUrl && <a href={data.videoUrl} className="rounded px-3 py-3 font-medium text-primary underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-primary">Watch the video</a>}
          </div>
          <p className="mt-4 text-sm text-ink-muted">Meet the fictional Rivera family. Judges can find login details in the submission.</p>
        </div>
        <PorchScene slot="wide" className="aspect-[2/1] w-full overflow-hidden rounded-3xl" />
      </section>
      <section aria-labelledby="homepage-preview" className="mx-auto max-w-7xl px-6 pb-16 sm:px-10">
        <h2 id="homepage-preview" className="text-3xl">A home for the everyday.</h2>
        <p className="mt-3 text-ink-soft">Maya’s page in the fictional Rivera family demo.</p>
        <img src="/demo-homepage.png" alt="Maya’s fictional family homepage with today’s plans, daily chores, weather and personalized cards." width="1440" height="1050" className="mt-6 w-full rounded-2xl border border-line shadow-card" />
      </section>
      <section aria-label="What you can do" className="mx-auto grid max-w-7xl gap-6 px-6 pb-20 sm:px-10 md:grid-cols-3">
        {[
          ['Make room for your interests', 'Describe a card you’d like to see, then arrange your homepage around what matters to you. Everyone gets their own page.'],
          ['Share the everyday jobs', 'Parents assign daily chores. Kids check them off, and everyone can see their own next step. Parents can follow the family’s progress.'],
          ['Explore, with parents nearby', 'Kids can ask questions and work through homework with hints. Parents can review conversations and follow up on flagged moments.'],
        ].map(([title, description]) => <article key={title} className="rounded-3xl border border-line bg-surface p-7"><h2 className="text-2xl">{title}</h2><p className="mt-4 leading-relaxed text-ink-soft">{description}</p></article>)}
      </section>
    </main>
    <footer className="border-t border-line px-6 py-8 text-center text-sm text-ink-muted">Kinlight · Built for a family, with room to grow.</footer>
  </div>
}
