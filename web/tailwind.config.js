/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Compass primary palette
        primary: {
          DEFAULT: '#007AFF',
          hover: '#0066D6',
          light: '#E8F3FF',
        },
        // Success / online
        green: {
          DEFAULT: '#05C46B',
          hover: '#049A54',
        },
        // Error
        danger: {
          DEFAULT: '#FF6A64',
          hover: '#FF453D',
        },
        // Warning
        warning: '#FF8A00',
        // Text
        txt: {
          DEFAULT: '#333E49',
          secondary: '#677380',
          placeholder: '#B4B4B4',
        },
        // Surfaces
        surface: {
          DEFAULT: '#F8F8F8',
          light: '#F5F5F5',
          white: '#FFFFFF',
          border: '#F0F0F0',
        },
        // Dark theme (Compass-style charcoal)
        dark: {
          bg: '#1C1C1C',
          elevated: '#252526',
          hover: '#2C2C2E',
          border: '#3C3C3E',
          muted: '#8E8E93',
        },
        // Sidebar (same charcoal as main)
        sidebar: {
          DEFAULT: '#1C1C1C',
          hover: '#252526',
          active: '#2e4038', // подсветка активного чата (тёмный зеленовато-серый, как на референсе)
          border: '#3C3C3E',
          text: '#9DA0B3',
        },
        // Nav rail
        nav: {
          DEFAULT: '#18181A',
          hover: '#252526',
          active: '#2C2C2E',
        },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'BlinkMacSystemFont', 'sans-serif'],
      },
      fontSize: {
        'sidebar-title': ['15px', { lineHeight: '1.35' }],
        'sidebar-name': ['14px', { lineHeight: '1.35' }],
        'sidebar-sub': ['12px', { lineHeight: '1.35' }],
        'sidebar-tab': ['12px', { lineHeight: '1.3' }],
      },
      borderRadius: {
        compass: '8px',
      },
      boxShadow: {
        compass: '0px 2px 15px 0px rgba(0, 0, 0, 0.05)',
        'compass-dialog': '0 0 0 3px rgba(0, 0, 0, 0.05)',
        'compass-lg': '0 8px 32px rgba(0, 0, 0, 0.12)',
      },
    },
  },
  plugins: [],
};
