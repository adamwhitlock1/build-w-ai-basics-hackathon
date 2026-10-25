import { ShieldIcon } from '../widgets/icons'

/** The notice every kid chat screen shows (spec: "The parent notice is always shown"). */
export function ChatNotice({ className = '' }: { className?: string }) {
  return (
    <p className={`inline-flex items-center gap-1.5 rounded-full bg-primary-soft px-3 py-1 text-sm text-primary ${className}`}>
      <ShieldIcon className="size-4 shrink-0" />
      Your parents can review these chats
    </p>
  )
}
