import { createApp } from 'vue'
import App from './App.vue'
import { BackendKey } from './backend/inject'
import { wailsBackend } from './backend/wails'
import './style.css'

createApp(App).provide(BackendKey, wailsBackend).mount('#app')
