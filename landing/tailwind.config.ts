import type { Config } from 'tailwindcss'

const config: Config = {
  content: ['./app/**/*.{js,ts,jsx,tsx}', './components/**/*.{js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        bunny: {
          50: '#eef5fc', 100: '#d9e8f8', 200: '#b3d1f1',
          300: '#80b4e8', 400: '#5a9de0', 500: '#4a90d9',
          600: '#3a7bc8', 700: '#2e62a3', 800: '#264f82',
          900: '#1f3f68', 950: '#122540',
        },
      },
      keyframes: {
        'demo-flow': {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-2px)' },
        },
      },
      animation: {
        'demo-flow': 'demo-flow 2s ease-in-out infinite',
      },
    },
  },
  plugins: [],
}
export default config
