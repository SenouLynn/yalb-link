/**
 * memoryStorage is an ordinary in-memory Storage for tests.
 *
 * It exists because the worksheet takes the store it uses as an argument, so a
 * test can watch exactly what a save wrote without depending on whatever
 * localStorage the environment happens to provide.
 */
export function memoryStorage(): Storage {
  const entries = new Map<string, string>()
  return {
    get length() {
      return entries.size
    },
    clear() {
      entries.clear()
    },
    getItem(key: string) {
      return entries.get(key) ?? null
    },
    key(index: number) {
      return [...entries.keys()][index] ?? null
    },
    removeItem(key: string) {
      entries.delete(key)
    },
    setItem(key: string, value: string) {
      entries.set(key, value)
    },
  }
}
