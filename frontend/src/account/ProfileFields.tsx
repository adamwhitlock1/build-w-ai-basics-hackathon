import type { Profile } from '../api/client'
import { TextAreaField, TextField } from '../components/Field'
import { PROFILE_MAX_LENGTH } from './forms'

interface ProfileFieldsProps {
  value: Profile
  onChange: (next: Profile) => void
}

/**
 * The "interests" and "where are you" fields, shared by the create-family
 * intro and the interests editor. Both may be left blank. The inputs are
 * named `interests` and `location`, so focusField can find them.
 */
export function ProfileFields({ value, onChange }: ProfileFieldsProps) {
  return (
    <>
      <TextAreaField
        label="What would you like to see on your page?"
        hint="Your interests, in your own words. We’ll use them to pick articles and events for you."
        name="interests"
        placeholder="e.g. space launches, Premier League soccer, easy weeknight dinners"
        maxLength={PROFILE_MAX_LENGTH}
        value={value.interests}
        onChange={(e) => onChange({ ...value, interests: e.target.value })}
      />
      <TextField
        label="Where are you?"
        hint="A city is fine. It’s for finding events near you."
        name="location"
        placeholder="e.g. Denver, Colorado"
        autoComplete="address-level2"
        maxLength={PROFILE_MAX_LENGTH}
        value={value.location}
        onChange={(e) => onChange({ ...value, location: e.target.value })}
      />
    </>
  )
}
