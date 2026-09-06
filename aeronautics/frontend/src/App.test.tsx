import { renderToStaticMarkup } from 'react-dom/server'
import { expect, test } from 'vitest'
import { App } from './App'

test('the standalone shell renders with its current capability and source', () => {
  const html = renderToStaticMarkup(<App />)
  expect(html).toContain('<h1 id="title">Aircraft design worksheet</h1>')
  expect(html).toContain('Project shell')
  expect(html).toContain('parameters are not available yet.')
  expect(html).toContain('https://computationaldesignlab.github.io/aircraft-design/intro.html')
})
