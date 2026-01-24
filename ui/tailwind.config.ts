import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: 'class',
  content: [
    "./pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      animation: {
        'demo-flow': 'demo-flow 2s ease-in-out infinite',
      },
      keyframes: {
        'demo-flow': {
          '0%': { width: '30%', opacity: '0.7' },
          '50%': { width: '80%', opacity: '1' },
          '100%': { width: '30%', opacity: '0.7' },
        },
      },
      colors: {
        bunny: {
          50: '#eef5fc',
          100: '#d9e8f8',
          200: '#b3d1f1',
          300: '#80b4e8',
          400: '#5a9de0',
          500: '#4a90d9',
          600: '#3a7bc8',
          700: '#2e62a3',
          800: '#264f82',
          900: '#1f3f68',
          950: '#122540',
        },
      },
    },
  },
  plugins: [],
};
export default config;
