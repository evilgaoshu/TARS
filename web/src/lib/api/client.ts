import axios from 'axios';
import { clearStoredSession, getStoredToken } from '../auth/storage';

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  timeout: 10000,
});

api.interceptors.request.use(config => {
  const token = getStoredToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  response => response,
  error => {
    if (error.response?.status === 401) {
      const requestURL = String(error.config?.url || '');
      if (requestURL.includes('/setup/wizard') || requestURL.includes('/bootstrap/status')) {
        return Promise.reject(error);
      }
      clearStoredSession();
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);
