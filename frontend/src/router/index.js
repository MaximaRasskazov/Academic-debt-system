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
import ProfilePage from '../views/ProfilePage.vue'

// Роли — slug'и backend'а (lowercase). admin везде имеет доступ как высший.
const routes = [
  { path: '/', redirect: '/debts' },
  { path: '/login', component: LoginPage, meta: { guest: true } },
  { path: '/register', component: RegisterPage, meta: { guest: true } },
  { path: '/debts', component: DebtsPage, meta: { auth: true } },
  { path: '/retakes', component: RetakesPage, meta: { auth: true } },
  { path: '/retakes/create', component: RetakeCreatePage, meta: { auth: true, roles: ['dean', 'admin'] } },
  { path: '/requests', component: RequestsPage, meta: { auth: true, roles: ['dean', 'admin'] } },
  { path: '/dean', component: DeanPage, meta: { auth: true, roles: ['dean', 'admin'] } },
  { path: '/teacher', component: TeacherPage, meta: { auth: true, roles: ['teacher', 'admin'] } },
  { path: '/teacher-requests', component: TeacherRequestsPage, meta: { auth: true, roles: ['teacher', 'dean', 'admin'] } },
  { path: '/statements', component: StatementsPage, meta: { auth: true, roles: ['teacher', 'dean', 'admin'] } },
  { path: '/profile', component: ProfilePage, meta: { auth: true } },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// getHomeForRole — куда отправлять пользователя после успешного логина,
// если в query нет ?next=. Импортируется из LoginPage.
export function getHomeForRole(role) {
  switch (role) {
    case 'admin':
    case 'dean':
      return '/dean'
    case 'teacher':
      return '/teacher'
    case 'student':
      return '/debts'
    default:
      return '/debts'
  }
}

router.beforeEach((to) => {
  const auth = useAuthStore()

  if (to.meta.auth && !auth.isLoggedIn) {
    // Кладём исходный путь в ?next=, чтобы после успешного логина вернуть
    // пользователя именно туда, куда он шёл.
    return { path: '/login', query: { next: to.fullPath } }
  }

  if (to.meta.guest && auth.isLoggedIn) {
    return getHomeForRole(auth.primaryRole)
  }

  if (to.meta.roles && !to.meta.roles.some((r) => auth.roleSlugs.includes(r))) {
    return getHomeForRole(auth.primaryRole)
  }
})

export default router
