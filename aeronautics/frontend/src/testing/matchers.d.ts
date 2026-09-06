// The jest-dom matchers are registered at run time by vitest.setup.ts. This
// file is what tells the compiler about them, because that setup file belongs
// to the Node project and the tests belong to the app one.
import '@testing-library/jest-dom/vitest'
