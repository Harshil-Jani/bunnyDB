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
          50: '#fef7ee',
          100: '#fdedd3',
          200: '#fad7a5',
          300: '#f7ba6d',
          400: '#f39333',
          500: '#f0750f',
          600: '#e15a09',
          700: '#ba430a',
          800: '#943610',
          900: '#782f11',
          950: '#411506',
        },
      },
    },
  },
  plugins: [],
};
export default config;
