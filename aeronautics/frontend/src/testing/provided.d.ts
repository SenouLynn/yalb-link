// The global setup starts the Go service and provides its address. Declaring it
// here rather than in the setup file puts it in the same TypeScript project as
// the tests that read it.
declare module 'vitest' {
  interface ProvidedContext {
    aeroBaseUrl: string
  }
}
export {}
