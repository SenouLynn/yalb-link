// Finding the service's complaint about one field.
//
// Two different things can complain about the same entry, and they are not the
// same kind of news. A refused command means the edit did not happen and the
// text is still in the box; a definition issue means the edit was accepted and
// the design is incomplete or inconsistent because of it. Both are shown on the
// field they belong to, and the refusal is shown first because it is the one
// the builder has to act on to make any progress at all.

import type { WorksheetApi } from '../useWorksheet.ts'

/** issueFor finds the service's complaint about one field, if there is one. */
export function issueFor(api: WorksheetApi, field: string): string | undefined {
  const failure = api.worksheet.failure
  if (failure === null) return undefined
  const issue = failure.issues.find((entry) => entry.field === field || entry.field.endsWith(`.${field}`))
  return issue?.detail
}

/** definitionIssue finds a structural complaint the evaluation reported. */
export function definitionIssue(api: WorksheetApi, field: string): string | undefined {
  const current = api.worksheet.current
  if (current === null) return undefined
  const all = [
    ...current.evaluation.definitionIssues,
    ...current.evaluation.geometryIssues,
  ]
  return all.find((issue) => issue.field === field)?.detail
}

/** anyIssue is the refusal if there is one, and the definition issue otherwise. */
export function anyIssue(api: WorksheetApi, field: string): string | undefined {
  return issueFor(api, field) ?? definitionIssue(api, field)
}
