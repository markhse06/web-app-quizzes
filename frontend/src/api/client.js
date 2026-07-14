const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

const ACCESS_TOKEN_KEY = 'access_token'
const REFRESH_TOKEN_KEY = 'refresh_token'

export class ApiError extends Error {
  constructor(status, message) {
    super(message)
    this.status = status
  }
}

export function saveTokens({ access_token: accessToken, refresh_token: refreshToken }) {
  localStorage.setItem(ACCESS_TOKEN_KEY, accessToken)
  localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken)
}

export function clearTokens() {
  localStorage.removeItem(ACCESS_TOKEN_KEY)
  localStorage.removeItem(REFRESH_TOKEN_KEY)
}

export function hasAccessToken() {
  return Boolean(localStorage.getItem(ACCESS_TOKEN_KEY))
}

async function refreshAccessToken() {
  const refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY)
  if (!refreshToken) return false

  const response = await fetch(`${API_URL}/api/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken }),
  })
  if (!response.ok) return false

  saveTokens(await response.json())
  return true
}

export async function apiRequest(path, options = {}, retryAfterRefresh = true) {
  const headers = new Headers(options.headers)
  const accessToken = localStorage.getItem(ACCESS_TOKEN_KEY)
  if (accessToken) headers.set('Authorization', `Bearer ${accessToken}`)
  if (options.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')

  const response = await fetch(`${API_URL}${path}`, { ...options, headers })
  if (response.status === 401 && retryAfterRefresh && await refreshAccessToken()) {
    return apiRequest(path, options, false)
  }
  if (response.status === 401) clearTokens()
  if (!response.ok) {
    const payload = await response.json().catch(() => ({}))
    throw new ApiError(response.status, payload.error ?? 'Не удалось выполнить запрос')
  }
  if (response.status === 204) return null
  return response.json()
}

export const authApi = {
  register: (email, password) => apiRequest('/api/user/register', {
    method: 'POST', body: JSON.stringify({ email, password }),
  }),
  login: (email, password) => apiRequest('/api/user/login', {
    method: 'POST', body: JSON.stringify({ email, password }),
  }),
}
