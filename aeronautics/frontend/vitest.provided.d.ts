// The address the global setup provides, declared for the Node project as well
// as the app one so both ends of the channel are typed.
declare module 'vitest' {
  interface ProvidedContext {
    aeroBaseUrl: string
  }
}
export {}
