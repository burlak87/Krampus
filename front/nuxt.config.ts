import tailwindcss from "@tailwindcss/vite";


export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  modules: ['@pinia/nuxt'],
  css: ['./app/assets/css/main.css'],
  imports: {
    dirs: [
      'types/**',
      'types/*',
      'shemas/**'
    ]
  },
  vite: {
    plugins: [
      tailwindcss(),
    ]
  }
})