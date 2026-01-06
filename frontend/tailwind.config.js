/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Apple System Colors
        'apple-blue': '#007AFF',
        'apple-orange': '#FF9500',
        'apple-green': '#34C759',
        'apple-red': '#FF3B30',
        'apple-purple': '#AF52DE',
        'apple-pink': '#FF2D55',
        'apple-teal': '#5AC8FA',
        'apple-indigo': '#5856D6',
      },
      backdropBlur: {
        '2xl': '40px',
        '3xl': '64px',
      },
      borderRadius: {
        '4xl': '32px',
      },
      boxShadow: {
        'glass': '0 8px 32px 0 rgba(31, 38, 135, 0.15)',
        'glass-lg': '0 25px 50px -12px rgba(0, 0, 0, 0.25)',
        'glow-blue': '0 0 40px rgba(0, 122, 255, 0.3)',
        'glow-orange': '0 0 40px rgba(255, 149, 0, 0.3)',
      },
      animation: {
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'float': 'float 6s ease-in-out infinite',
      },
      keyframes: {
        float: {
          '0%, 100%': { transform: 'translateY(0px)' },
          '50%': { transform: 'translateY(-10px)' },
        }
      }
    },
  },
  plugins: [],
}
