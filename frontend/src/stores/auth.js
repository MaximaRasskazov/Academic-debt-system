import { defineStore } from 'pinia'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('token') || null,
    role: localStorage.getItem('role') || null,
    user: JSON.parse(localStorage.getItem('user') || 'null'),
  }),

  getters: {
    isLoggedIn: (state) => !!state.token,
    isStudent: (state) => state.role === 'STUDENT',
    isTeacher: (state) => state.role === 'TEACHER',
    isDean: (state) => state.role === 'DEAN',
  },

  actions: {
    login({ token, role, user }) {
      this.token = token
      this.role = role
      this.user = user

      localStorage.setItem('token', token)
      localStorage.setItem('role', role)
      localStorage.setItem('user', JSON.stringify(user))
    },

    logout() {
      this.token = null
      this.role = null
      this.user = null

      localStorage.removeItem('token')
      localStorage.removeItem('role')
      localStorage.removeItem('user')
    },
  },
})
