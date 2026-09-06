export function App() {
  return (
    <main>
      <header>
        <span className="wordmark">YALB / AERO</span>
        <span className="status">Project shell</span>
      </header>
      <section aria-labelledby="title">
        <p className="eyebrow">Fixed-wing RC aircraft</p>
        <h1 id="title">Aircraft design worksheet</h1>
        <p>
          The worksheet is under development. Calculations and editable design
          parameters are not available yet.
        </p>
        <p>
          The planned methods follow the{' '}
          <a href="https://computationaldesignlab.github.io/aircraft-design/intro.html">
            CODE Lab Aircraft Design book
          </a>
          .
        </p>
      </section>
    </main>
  )
}
