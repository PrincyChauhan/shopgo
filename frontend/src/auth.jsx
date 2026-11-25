import { createContext, useCallback, useContext, useEffect, useState } from 'react';
import { api } from './api';

const Ctx = createContext(null);
export const useAuth = () => useContext(Ctx);

// Holds who is logged in plus the cart badge number, for the whole app.
export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [ready, setReady] = useState(false);
  const [cartCount, setCartCount] = useState(0);

  const refreshCart = useCallback(async () => {
    try {
      const c = await api('/cart');
      setCartCount(c.item_count);
    } catch {
      setCartCount(0);
    }
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem('token');
    setUser(null);
    setCartCount(0);
  }, []);

  useEffect(() => {
    window.addEventListener('logout', logout);
    if (localStorage.getItem('token')) {
      api('/auth/me')
        .then((u) => { setUser(u); refreshCart(); })
        .catch(logout)
        .finally(() => setReady(true));
    } else {
      setReady(true);
    }
    return () => window.removeEventListener('logout', logout);
  }, [logout, refreshCart]);

  const login = (token, u) => {
    localStorage.setItem('token', token);
    setUser(u);
    refreshCart();
  };

  return (
    <Ctx.Provider value={{ user, ready, login, logout, cartCount, refreshCart }}>
      {children}
    </Ctx.Provider>
  );
}
