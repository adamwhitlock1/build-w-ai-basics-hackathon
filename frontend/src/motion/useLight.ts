// One lit thing per screen (brand/lantern-ux.md): everything that wants the
// full-strength glow claims it here, and holder() in light.ts picks one.
// The claims live in this module, the one place every component on the
// screen shares, and components read the winner through
// useSyncExternalStore, so only the ones whose answer changes re-render.
import { useEffect, useId, useSyncExternalStore } from 'react'
import { holder, type LightClaim, type LightKind } from './light'

const claims = new Map<string, LightClaim>()
const listeners = new Set<() => void>()
let current: string | undefined

function settle() {
  const next = holder([...claims.values()])
  if (next === current) return
  current = next
  listeners.forEach((listener) => listener())
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

const getHolder = () => current

/**
 * Claims the screen's one full-strength glow while `active`, and says
 * whether this component holds it. A component that doesn't hold it still
 * shows its words, with its light dimmed.
 */
export function useLight(kind: LightKind, active: boolean): boolean {
  const id = useId()
  useEffect(() => {
    if (!active) return
    claims.set(id, { id, kind, since: Date.now() })
    settle()
    return () => {
      claims.delete(id)
      settle()
    }
  }, [id, kind, active])
  return useSyncExternalStore(subscribe, getHolder) === id && active
}
