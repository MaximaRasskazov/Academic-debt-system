import axios from 'axios'

const http = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? 'http://localhost:8080',
  withCredentials: true, // нужен для httpOnly refresh-cookie
})

// Прикрепляем access-token к каждому запросу
http.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

// При 401 пробуем один раз обновить токен и повторить запрос
let refreshing = false
let refreshQueue = []

http.interceptors.response.use(
  (res) => res,
  async (err) => {
    const original = err.config
    if (err.response?.status !== 401 || original._retry) {
      return Promise.reject(err)
    }
    original._retry = true

    if (refreshing) {
      return new Promise((resolve, reject) => {
        refreshQueue.push({ resolve, reject, config: original })
      })
    }

    refreshing = true
    try {
      const { data } = await axios.post(
        `${import.meta.env.VITE_API_URL ?? 'http://localhost:8080'}/api/auth/refresh`,
        {},
        { withCredentials: true },
      )
      const newToken = data.access_token
      localStorage.setItem('token', newToken)

      // обновляем роль/юзера если бэк вернул
      if (data.user) localStorage.setItem('user', JSON.stringify(data.user))

      refreshQueue.forEach(({ resolve, config }) => {
        config.headers.Authorization = `Bearer ${newToken}`
        resolve(http(config))
      })
      refreshQueue = []

      original.headers.Authorization = `Bearer ${newToken}`
      return http(original)
    } catch {
      refreshQueue.forEach(({ reject }) => reject(err))
      refreshQueue = []
      localStorage.clear()
      window.location.href = '/login'
      return Promise.reject(err)
    } finally {
      refreshing = false
    }
  },
)

export default http
