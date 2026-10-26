import { createApp } from 'vue'
import App from './App.vue'
import { BackendKey } from './backend/inject'
import { wailsBackend } from './backend/wails'
import { router } from './router'
import './style.css'

createApp(App).use(router).provide(BackendKey, wailsBackend).mount('#app')
