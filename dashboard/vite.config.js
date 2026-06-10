import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
    root: 'src/app',
    plugins: [react()],
    build: {
        outDir: '../../dist',
        emptyOutDir: true,
    },
    server: {
        port: 5173,
        proxy: {
            '/api': {
                target: 'http://127.0.0.1:8080', 
                rewrite: (path) => path.replace(/^\/api/, ''),
                changeOrigin: true,
            },
            '/ws': {
                target: 'ws://127.0.0.1:8080',   
                ws: true,
                changeOrigin: true,
            },
        },
    },
})