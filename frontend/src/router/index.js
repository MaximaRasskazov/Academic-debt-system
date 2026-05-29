import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

import LoginPage from '../views/LoginPage.vue'
import RegisterPage from '../views/RegisterPage.vue'
import StudentPage from '../views/StudentPage.vue'
import RetakesPage from '../views/RetakesPage.vue'
import RetakeCreatePage from '../views/RetakeCreatePage.vue'
import RequestsPage from '../views/RequestsPage.vue'
import DeanPage from '../views/DeanPage.vue'
import TeacherPage from '../views/TeacherPage.vue'
import TeacherRequestsPage from '../views/TeacherRequestsPage.vue'
import StatementsPage from '../views/StatementsPage.vue'
import UsersPage from '../views/UsersPage.vue'

const HOME = { STUDENT: '/student', TEACHER: '/teacher', DEAN: '/dean' }
const homeFor = (role) => HOME[role] ?? '/login'

const routes = [
  { path: '/', redirect: () => homeFor(useAuthStore().primaryRole) },
  { path: '/login', component: LoginPage, meta: { guest: true } },
  { path: '/register', component: RegisterPage, meta: { guest: true } },
  { path: '/student', component: StudentPage, meta: { auth: true, roles: ['student'] } },
  { path: '/retakes', component: RetakesPage, meta: { auth: true } },
  { path: '/retakes/create', component: RetakeCreatePage, meta: { auth: true, roles: ['dean'] } },
  { path: '/requests', component: RequestsPage, meta: { auth: true, roles: ['dean'] } },
  { path: '/dean', component: DeanPage, meta: { auth: true, roles: ['dean'] } },
  { path: '/teacher', component: TeacherPage, meta: { auth: true, roles: ['teacher'] } },
  { path: '/teacher-requests', component: TeacherRequestsPage, meta: { auth: true, roles: ['teacher'] } },
  { path: '/statements', component: StatementsPage, meta: { auth: true, roles: ['teacher', 'dean'] } },
  { path: '/users', component: UsersPage, meta: { auth: true } },
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
    return homeFor(auth.primaryRole)
  }

  if (to.meta.roles && !to.meta.roles.includes(auth.primaryRole)) {
    return homeFor(auth.primaryRole)
  }
})

export default router
