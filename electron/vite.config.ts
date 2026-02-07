import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'path'

export default defineConfig({
    root: 'src/renderer',
    base: './',
    build: {
        outDir: '../../out/renderer',
        emptyOutDir: true
    },
    resolve: {
        alias: {
            '@renderer': resolve('src/renderer/src')
        }
    },
    plugins: [react()]
})
