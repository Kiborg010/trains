import React, { createContext, useState, useCallback, useEffect } from 'react';

const AuthContext = createContext(null);
const API_BASE_URL = 'http://localhost:8080/api';

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [token, setToken] = useState(null);
  const [loading, setLoading] = useState(true);

  // Restore session from token in localStorage
  useEffect(() => {
    let cancelled = false;

    async function restoreSession() {
      const savedToken = localStorage.getItem('authToken');
      const isTokenValid =
        savedToken &&
        savedToken !== 'undefined' &&
        savedToken !== 'null' &&
        savedToken.length > 10;

      if (!isTokenValid) {
        localStorage.removeItem('authToken');
        if (!cancelled) {
          setToken(null);
          setUser(null);
          setLoading(false);
        }
        return;
      }

      try {
        const response = await fetch(`${API_BASE_URL}/auth/me`, {
          method: 'GET',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${savedToken}`,
          },
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        const data = await response.json();
        if (!data?.ok || !data?.user) {
          throw new Error('Некорректный ответ /auth/me');
        }

        if (!cancelled) {
          setToken(savedToken);
          setUser(data.user);
        }
      } catch (error) {
        console.error('Session restore failed:', error);
        localStorage.removeItem('authToken');
        if (!cancelled) {
          setToken(null);
          setUser(null);
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    restoreSession();

    return () => {
      cancelled = true;
    };
  }, []);

  const login = useCallback(async (email, password) => {
    try {
      const response = await fetch(`${API_BASE_URL}/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });

      if (!response.ok) {
        const err = await response.json();
        throw new Error(err.error || 'Не удалось войти');
      }

      const data = await response.json();
      if (data.ok && data.token && data.user) {
        localStorage.setItem('authToken', data.token);
        setToken(data.token);
        setUser(data.user);
        return { ok: true, user: data.user, token: data.token };
      }

      throw new Error('Некорректный ответ сервера');
    } catch (error) {
      return { ok: false, error: error.message };
    }
  }, []);

  const register = useCallback(async (email, password) => {
    try {
      const response = await fetch(`${API_BASE_URL}/auth/register`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });

      if (!response.ok) {
        const err = await response.json();
        throw new Error(err.error || 'Не удалось зарегистрироваться');
      }

      const data = await response.json();
      if (data.ok && data.user) {
        setUser(data.user);
        return { ok: true, user: data.user };
      }

      throw new Error('Некорректный ответ сервера');
    } catch (error) {
      return { ok: false, error: error.message };
    }
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem('authToken');
    setToken(null);
    setUser(null);
  }, []);

  const value = {
    user,
    token,
    loading,
    login,
    register,
    logout,
    isAuthenticated: !!token && !!user,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = React.useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth должен использоваться внутри AuthProvider');
  }
  return context;
}
