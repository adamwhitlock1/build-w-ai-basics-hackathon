import { useId, type InputHTMLAttributes, type ReactNode, type TextareaHTMLAttributes } from 'react'

interface FieldProps {
  label: string
  /** Under the label: what to enter, or why we ask. */
  hint?: ReactNode
  /** Under the control after a failed submit: what to fix. */
  error?: string
  /** Adds a quiet "(optional)" after the label. */
  optional?: boolean
}

/** The props Field hands to its control so the label, hint and error are announced with it. */
interface ControlProps {
  id: string
  'aria-describedby'?: string
  'aria-invalid'?: true
  className: string
}

type TextFieldProps = FieldProps & Omit<InputHTMLAttributes<HTMLInputElement>, keyof ControlProps>
type TextAreaFieldProps = FieldProps & Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, keyof ControlProps>

/** A labelled single-line input with an optional hint and an inline error. */
export function TextField({ label, hint, error, optional, ...inputProps }: TextFieldProps) {
  return (
    <Field label={label} hint={hint} error={error} optional={optional}>
      {(control) => <input {...inputProps} {...control} className={`h-12 ${control.className}`} />}
    </Field>
  )
}

/** A labelled multi-line input with an optional hint and an inline error. */
export function TextAreaField({ label, hint, error, optional, rows = 3, ...textareaProps }: TextAreaFieldProps) {
  return (
    <Field label={label} hint={hint} error={error} optional={optional}>
      {(control) => (
        <textarea {...textareaProps} {...control} rows={rows} className={`resize-y py-3 ${control.className}`} />
      )}
    </Field>
  )
}

/**
 * Lays out label, hint, control and error, and links them by id. The control
 * is a render prop (a function of the ids) so inputs and textareas share it.
 */
function Field({
  label,
  hint,
  error,
  optional,
  children,
}: FieldProps & { children: (control: ControlProps) => ReactNode }) {
  const id = useId()
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  const describedBy = [error && errorId, hint && hintId].filter(Boolean).join(' ')

  return (
    <div>
      <label htmlFor={id} className="block font-medium text-ink">
        {label}
        {optional && <span className="font-normal text-ink-muted"> (optional)</span>}
      </label>
      {hint && (
        <p id={hintId} className="mt-1 text-sm leading-relaxed text-ink-muted">
          {hint}
        </p>
      )}
      <div className="mt-2">
        {children({
          id,
          'aria-describedby': describedBy || undefined,
          'aria-invalid': error ? true : undefined,
          className: `w-full rounded-2xl border bg-bg px-4 text-base text-ink placeholder:text-ink-muted transition-colors focus:bg-surface focus:outline-none focus:ring-4 ${
            error
              ? 'border-danger/50 focus:border-danger/60 focus:ring-danger/10'
              : 'border-line focus:border-primary/50 focus:ring-primary/10'
          }`,
        })}
      </div>
      {error && (
        <p id={errorId} className="mt-2 text-sm font-medium text-danger">
          {error}
        </p>
      )}
    </div>
  )
}
