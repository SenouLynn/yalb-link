import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { UserEvent } from '@testing-library/user-event'
import { expect, inject } from 'vitest'
import { httpTransport } from '../api/client.ts'
import { memoryStorage } from './storage.ts'
import { Worksheet } from '../Worksheet.tsx'

// The shared harness for the browser tests that run against the real Go
// service. It is here rather than duplicated per file so that "settled" means
// the same thing everywhere: the worksheet is showing a result that describes
// the design as it now stands.

/** renderWorksheet mounts the worksheet against the running service. */
export function renderWorksheet(storage: Storage = memoryStorage()): UserEvent {
  const base = inject('aeroBaseUrl')
  render(<Worksheet transport={httpTransport(base)} session="task07-test" storage={storage} />)
  return userEvent.setup()
}

/** settled waits for the worksheet to be showing a current result. */
export async function settled(): Promise<void> {
  await waitFor(
    () => {
      expect(screen.getByText('Current')).toBeInTheDocument()
    },
    { timeout: 15_000 },
  )
}

/** enter commits a value into a field and waits for the result to settle. */
export async function enter(user: UserEvent, label: string, text: string): Promise<void> {
  const field = screen.getByLabelText(label)
  await user.clear(field)
  await user.type(field, `${text}{Enter}`)
  await settled()
}

/** setSelect chooses an option and waits. */
export async function setSelect(user: UserEvent, label: string, value: string): Promise<void> {
  await user.selectOptions(screen.getByLabelText(label), value)
  await settled()
}

/** press clicks a button by its accessible name and waits. */
export async function press(user: UserEvent, name: string | RegExp): Promise<void> {
  await user.click(screen.getByRole('button', { name }))
  await settled()
}

/** buildBaseCandidate enters the shared fixture: 2 kg, CLmax 1.2, span and AR. */
export async function buildBaseCandidate(user: UserEvent): Promise<void> {
  await enter(user, 'All-up mass', '2')
  await enter(user, 'Where it comes from', 'target all-up mass')
  await enter(user, 'Whole-aircraft CLmax', '1.2')
  await enter(user, 'Where the coefficient comes from', 'assumed for initial sizing')
  await enter(user, 'Span', '1.2')
  await enter(user, 'Aspect ratio', '6')
}

/** section returns the panel with the given heading, so a query says which of
 * the worksheet's tables or drawings it means. */
export function section(title: string): HTMLElement {
  const heading = screen.getByText(title)
  const panel = heading.closest('details')
  if (panel === null) throw new Error(`no section titled ${title}`)
  return panel
}

/** inSection scopes a query to one panel. */
export function inSection(title: string) {
  return within(section(title))
}

/** open expands a collapsed subsection by clicking its summary. */
export async function open(user: UserEvent, title: string): Promise<void> {
  await user.click(screen.getByText(title))
}

/**
 * componentPanel returns one listed component's editor. Every component uses
 * the same field labels, so a query has to say which one it means.
 */
export function componentPanel(name: string): HTMLElement {
  const label = screen.getByText(name, { selector: '.component-name' })
  const panel = label.closest('details')
  if (panel === null) throw new Error(`no editor for the component ${name}`)
  return panel
}

/** enterIn commits a value into a field inside one container. */
export async function enterIn(
  user: UserEvent,
  container: HTMLElement,
  label: string,
  text: string,
): Promise<void> {
  const field = within(container).getByLabelText(label)
  await user.clear(field)
  await user.type(field, `${text}{Enter}`)
  await settled()
}

/** selectIn chooses an option inside one container. */
export async function selectIn(
  user: UserEvent,
  container: HTMLElement,
  label: string,
  value: string,
): Promise<void> {
  await user.selectOptions(within(container).getByLabelText(label), value)
  await settled()
}

/** rowFor returns the field row a labelled control belongs to, so a query can
 * name which of several identical buttons it means. */
export function rowFor(label: string): HTMLElement {
  const row = screen.getByLabelText(label).closest('.field')
  if (!(row instanceof HTMLElement)) throw new Error(`no field row for ${label}`)
  return row
}
