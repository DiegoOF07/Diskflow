import { createApp } from 'vue'
import App from './App.vue'
import { BackendKey } from './backend/inject'
import { wailsBackend } from './backend/wails'
import { router } from './router'
// Self-hosted: DiskFlow makes no network requests. Latin covers Spanish.
import '@fontsource/barlow/latin-400.css'
import '@fontsource/barlow/latin-500.css'
import '@fontsource/barlow/latin-600.css'
import '@fontsource/barlow-semi-condensed/latin-400.css'
import '@fontsource/barlow-semi-condensed/latin-600.css'
import './style.css'

createApp(App).use(router).provide(BackendKey, wailsBackend).mount('#app')
