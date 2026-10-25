import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { modeNames, useDeleteChat, useMemberTranscript } from '../../api/chat'
import { useFamilyMember } from '../../api/hooks'
import { MessageList } from '../../chat/MessageList'
import { ReviewsLayout } from '../../chat/ReviewsLayout'
import { FormError } from '../../components/FormError'
import { nameFor } from '../../family/people'

/** ?message= is a message to highlight, from the review queue. */
interface TranscriptSearch {
  message?: number
}

export const Route = createFileRoute('/_authed/reviews/$memberId/$chatId')({
  validateSearch: (search: Record<string, unknown>): TranscriptSearch => {
    const id = Number(search.message)
    return { message: Number.isSafeInteger(id) && id > 0 ? id : undefined }
  },
  component: TranscriptPage,
})

/** A kid's whole conversation, read-only, with flagged messages marked, and a way to delete it. */
function TranscriptPage() {
  const params = Route.useParams()
  const { message } = Route.useSearch()
  const memberId = Number(params.memberId)
  const chatId = Number(params.chatId)
  const member = useFamilyMember(memberId)
  const transcript = useMemberTranscript(memberId, chatId)
  const remove = useDeleteChat(memberId)
  const navigate = useNavigate()
  const [confirming, setConfirming] = useState(false)
  const name = member ? nameFor(member.email) : 'Kid'

  // Bring the highlighted message into view once it's on the page.
  useEffect(() => {
    if (transcript.data && message) {
      document.getElementById(`message-${message}`)?.scrollIntoView({ block: 'center' })
    }
  }, [transcript.data, message])

  async function deleteChat() {
    await remove.mutateAsync(chatId)
    await navigate({ to: '/reviews/$memberId', params: { memberId: String(memberId) } })
  }

  return (
    <ReviewsLayout back={{ to: `/reviews/${memberId}`, label: `${name}’s chats` }}>
      {transcript.isError && <FormError message={transcript.error.message} hint={transcript.error.hint} />}
      {transcript.data && (
        <>
          <header className="mb-6">
            <h1 className="text-3xl leading-tight break-words text-ink sm:text-4xl">
              {transcript.data.conversation.title || 'New chat'}
            </h1>
            <p className="mt-2 text-sm text-ink-soft">
              {name} · {modeNames[transcript.data.conversation.mode]} · {transcript.data.messages.length} messages
            </p>
          </header>
          <MessageList
            messages={transcript.data.messages}
            kidName={name}
            flags={transcript.data.flags}
            toolCalls={transcript.data.toolCalls}
            highlightId={message}
          />
          <div className="mt-10 border-t border-line pt-5">
            {confirming ? (
              <div className="flex flex-wrap items-center gap-3">
                <p className="text-ink">Delete this conversation for good? {name} won’t see it any more.</p>
                <button
                  type="button"
                  disabled={remove.isPending}
                  onClick={() => void deleteChat().catch(() => {})}
                  className="h-10 rounded-2xl bg-danger px-4 text-sm font-medium text-on-danger hover:opacity-90 disabled:opacity-60"
                >
                  Delete permanently
                </button>
                <button
                  type="button"
                  onClick={() => setConfirming(false)}
                  className="h-10 rounded-2xl px-3 text-sm font-medium text-ink-soft hover:text-ink"
                >
                  Cancel
                </button>
              </div>
            ) : (
              <button
                type="button"
                onClick={() => setConfirming(true)}
                className="text-sm font-medium text-danger underline-offset-4 hover:underline"
              >
                Delete conversation…
              </button>
            )}
            {remove.isError && <FormError message={remove.error.message} hint={remove.error.hint} />}
          </div>
        </>
      )}
    </ReviewsLayout>
  )
}
