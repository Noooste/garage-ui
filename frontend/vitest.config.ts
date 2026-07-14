import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    coverage: {
      provider: 'v8',
      include: [
        'src/lib/preview-utils.ts',
        'src/lib/highlight.ts',
        'src/hooks/useObjectPreview.ts',
        'src/components/buckets/ObjectPreview.tsx',
      ],
      thresholds: { statements: 90, branches: 90, functions: 90, lines: 90 },
    },
  },
});
