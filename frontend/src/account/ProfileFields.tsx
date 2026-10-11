import type { Profile } from '../api/client'
import { TextAreaField, TextField } from '../components/Field'
import { PROFILE_MAX_LENGTH } from './forms'

interface ProfileFieldsProps {
  value: Profile
  onChange: (next: Profile) => void
  /**
   * Whose interests these are: your own (the default), or a family member's
   * that a parent is setting for them. Only the wording changes.
   */
  whose?: 'yours' | 'theirs'
}

/** The fields' wording, for your own interests or someone else's. */
const COPY = {
  yours: {
    interestsLabel: 'What would you like to see on your page?',
    interestsHint: 'Your interests, in your own words. We’ll use them to pick articles and events for you.',
    locationLabel: 'Where are you?',
    locationHint: 'A city is fine. It’s for finding events near you.',
  },
  theirs: {
    interestsLabel: 'What would they like to see on their page?',
    interestsHint: 'Their interests, in plain words. We’ll use them to pick articles and events for them.',
    locationLabel: 'Where are they?',
    locationHint: 'A city is fine. It’s for finding events near them.',
  },
}

/**
 * The "interests" and "where are you" fields, shared by the create-family
 * intro and the interests editor. Both may be left blank. The inputs are
 * named `interests` and `location`, so focusField can find them.
 */
export function ProfileFields({ value, onChange, whose = 'yours' }: ProfileFieldsProps) {
  const copy = COPY[whose]
  return (
    <>
      <TextAreaField
        label={copy.interestsLabel}
        hint={copy.interestsHint}
        name="interests"
        placeholder="e.g. space launches, Premier League soccer, easy weeknight dinners"
        maxLength={PROFILE_MAX_LENGTH}
        value={value.interests}
        onChange={(e) => onChange({ ...value, interests: e.target.value })}
      />
      <TextField
        label={copy.locationLabel}
        hint={copy.locationHint}
        name="location"
        placeholder="e.g. Denver, Colorado"
        // The browser's saved address is yours, so only offer it for your own.
        autoComplete={whose === 'yours' ? 'address-level2' : 'off'}
        maxLength={PROFILE_MAX_LENGTH}
        value={value.location}
        onChange={(e) => onChange({ ...value, location: e.target.value })}
      />
    </>
  )
}
