import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    host: 'localhost',
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:18790',
        changeOrigin: true,
      },
      '/mcp': {
        target: 'http://127.0.0.1:18790',
        changeOrigin: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          react: ['react', 'react-dom', 'react-router-dom'],
          codemirror: ['@codemirror/lang-sql', '@codemirror/theme-one-dark', '@uiw/react-codemirror'],
          ui: ['lucide-react', 'class-variance-authority', 'clsx', 'tailwind-merge'],
        },
      },
    },
  },
  test: {
    environment: 'happy-dom',
    globals: true,
    setupFiles: './src/test/setup.ts',
  },
});
