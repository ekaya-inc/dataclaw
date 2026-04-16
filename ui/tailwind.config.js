/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        'brand-purple': 'var(--brand-purple)',
        'text-primary': 'var(--text-primary)',
        'text-secondary': 'var(--text-secondary)',
        'text-tertiary': 'var(--text-tertiary)',
        'surface-primary': 'var(--surface-primary)',
        'surface-secondary': 'var(--surface-secondary)',
        'surface-tertiary': 'var(--surface-tertiary)',
        'surface-active': 'var(--surface-active)',
        'surface-hover': 'var(--surface-hover)',
        'surface-submit': 'var(--surface-submit)',
        'surface-submit-hover': 'var(--surface-submit-hover)',
        'surface-destructive': 'var(--surface-destructive)',
        'surface-destructive-hover': 'var(--surface-destructive-hover)',
        'border-light': 'var(--border-light)',
        'border-medium': 'var(--border-medium)',
        'border-heavy': 'var(--border-heavy)',
      },
      fontFamily: {
        sans: ['DM Sans', 'Inter', 'sans-serif'],
        mono: ['Roboto Mono', 'monospace'],
      },
    },
  },
  plugins: [],
};
