/** @type {import('tailwindcss').Config} */
module.exports = {
    content: [
        "./src/renderer/index.html",
        "./src/renderer/src/**/*.{js,ts,jsx,tsx}",
    ],
    theme: {
        extend: {
            colors: {
                'neon-green': '#00ff41',
                'dark-bg': '#0a0a0a',
                'panel-bg': '#111111',
            },
            fontFamily: {
                'mono': ['Consolas', 'Monaco', 'monospace'],
                'sans': ['Inter', 'sans-serif'],
            }
        },
    },
    plugins: [],
}
