import React, { createContext, useState, useCallback, useEffect } from 'react';

const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [token, setToken] = useState(null);
  const [loading, setLoading] = useState(true);

  // Load token from localStorage on mount
  useEffect(() => {
    const savedToken = localStorage.getItem('authToken');
    console.log('🔍 Проверка localStorage, токен:', savedToken);
    
    // ✅ Защита от мусорных токенов
    if (savedToken && 
        savedToken !== 'undefined' && 
        savedToken !== 'null' && 
        savedToken.length > 10) {
      console.log('✅ Токен валидный, устанавливаю');
      setToken(savedToken);
    } else {
      // Если токен мусорный - удаляем его
      console.log('❌ Токен мусорный или отсутствует, очищаю');
      localStorage.removeItem('authToken');
      setToken(null);
    }
    
    setLoading(false);
  }, []);

  const login = useCallback(async (email, password) => {
    try {
      console.log('🔐 Попытка входа:', email);
      const response = await fetch('http://localhost:8080/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });

      if (!response.ok) {
        const err = await response.json();
        console.log('❌ Ошибка входа:', err);
        throw new Error(err.error || 'Login failed');
      }

      const data = await response.json();
      console.log('📦 Ответ от сервера:', data);
      
      if (data.ok && data.token && data.user) {
        console.log('✅ Успешный вход, сохраняю токен');
        localStorage.setItem('authToken', data.token);
        setToken(data.token);
        setUser(data.user);
        return { ok: true, user: data.user, token: data.token };
      }
      throw new Error('Invalid response');
    } catch (error) {
      console.error('❌ Ошибка:', error);
      return { ok: false, error: error.message };
    }
  }, []);

  const register = useCallback(async (email, password) => {
    try {
      console.log('📝 Попытка регистрации:', email);
      const response = await fetch('http://localhost:8080/api/auth/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });

      if (!response.ok) {
        const err = await response.json();
        console.log('❌ Ошибка регистрации:', err);
        throw new Error(err.error || 'Registration failed');
      }

      const data = await response.json();
      console.log('📦 Ответ от сервера:', data);
      
      if (data.ok && data.user) {
        console.log('✅ Успешная регистрация');
        setUser(data.user);
        return { ok: true, user: data.user };
      }
      throw new Error('Invalid response');
    } catch (error) {
      console.error('❌ Ошибка:', error);
      return { ok: false, error: error.message };
    }
  }, []);

  const logout = useCallback(() => {
    console.log('🚪 Выход из системы');
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
    isAuthenticated: !!token,  // !! превращает token в true/false
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = React.useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
}