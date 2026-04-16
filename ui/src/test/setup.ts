import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach, beforeEach, vi } from 'vitest';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

beforeEach(() => {
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: {
      href: 'http://localhost:5173/',
      origin: 'http://localhost:5173',
      pathname: '/',
      search: '',
      hash: '',
    },
  });

  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: {
      writeText: vi.fn().mockResolvedValue(undefined),
    },
  });
});
