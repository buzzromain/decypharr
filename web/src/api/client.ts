import axios from 'axios'

const apiBaseURL = (import.meta.env.VITE_API_URL as string | undefined) ?? '/api'
const serverBaseURL = apiBaseURL.replace(/\/api\/?$/, '') || ''

export const apiClient = axios.create({ baseURL: apiBaseURL, withCredentials: true })
export const serverClient = axios.create({ baseURL: serverBaseURL, withCredentials: true })

apiClient.interceptors.response.use(
  (res) => res,
  (err) => {
    const status = err.response?.status
    console.warn('[apiClient] error', status, err.config?.url)
    if (status === 401 && window.location.pathname !== '/login') {
      window.location.href = '/login'
    }
    return Promise.reject(err)
  }
)
