import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';

// Testing Library's automatic cleanup only self-registers when a global
// afterEach exists. This harness does not set test.globals, so register it
// explicitly to unmount rendered trees between tests.
afterEach(() => {
  cleanup();
});
