import { createRouter, createWebHashHistory } from 'vue-router'
import RootPage from './features/explorer/RootPage.vue'
import HomeView from './features/home/HomeView.vue'

// Hash history: the app is served from embedded files, never from a web
// server that could resolve other paths.
export const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    {
      path: '/root/:id(\\d+)',
      name: 'explorer',
      component: RootPage,
      props: (route) => ({ id: Number(route.params.id), scan: route.query.scan === '1' }),
    },
    { path: '/:rest(.*)*', redirect: '/' },
  ],
})
