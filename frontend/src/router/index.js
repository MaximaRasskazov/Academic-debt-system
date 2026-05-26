import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

import LoginPage from '../views/LoginPage.vue'
import RegisterPage from '../views/RegisterPage.vue'
import DebtsPage from '../views/DebtsPage.vue'
import RetakesPage from '../views/RetakesPage.vue'
import RetakeCreatePage from '../views/RetakeCreatePage.vue'
import RequestsPage from '../views/RequestsPage.vue'
import DeanPage from '../views/DeanPage.vue'
import TeacherPage from '../views/TeacherPage.vue'
import TeacherRequestsPage from '../views/TeacherRequestsPage.vue'
import StatementsPage from '../views/StatementsPage.vue'

const routes = [
  { path: '/', redirect: '/debts' },
  { path: '/login', component: LoginPage, meta: { guest: true } },
  { path: '/register', component: RegisterPage, meta: { guest: true } },
  { path: '/debts', component: DebtsPage, meta: { auth: true } },
  { path: '/retakes', component: RetakesPage, meta: { auth: true } },
  { path: '/retakes/create', component: RetakeCreatePage, meta: { auth: true, roles: ['DEAN'] } },
  { path: '/requests', component: RequestsPage, meta: { auth: true, roles: ['DEAN'] } },
  { path: '/dean', component: DeanPage, meta: { auth: true, roles: ['DEAN'] } },
  { path: '/teacher', component: TeacherPage, meta: { auth: true, roles: ['TEACHER'] } },
  { path: '/teacher-requests', component: TeacherRequestsPage, meta: { auth: true, roles: ['TEACHER'] } },
  { path: '/statements', component: StatementsPage, meta: { auth: true, roles: ['TEACHER', 'DEAN'] } },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  const auth = useAuthStore()

  if (to.meta.auth && !auth.isLoggedIn) {
    return '/login'
  }

  if (to.meta.guest && auth.isLoggedIn) {
    return '/debts'
  }

  if (to.meta.roles && !to.meta.roles.includes(auth.role)) {
    return '/debts'
  }
})

export default router
