import { useEffect } from 'react';
import { Link, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { useAuth } from './auth.jsx';
import Shop from './pages/Shop.jsx';
import Product from './pages/Product.jsx';
import AuthForm from './pages/AuthForm.jsx';
import Cart from './pages/Cart.jsx';
import Orders from './pages/Orders.jsx';
import Admin from './pages/Admin.jsx';

function Nav() {
  const { user, logout, cartCount } = useAuth();
  const { pathname } = useLocation();
  const inAdmin = pathname.startsWith('/admin');

  if (inAdmin) {
    return (
      <header className="nav nav-admin">
        <Link to="/admin" className="brand brand-on-dark">🛍️ <b>ShopGo</b> <em>Admin</em></Link>
        <nav className="nav-links">
          <Link to="/">Open store</Link>
          {user && <span className="who">{user.name.split(' ')[0]}</span>}
          <button onClick={logout}>Logout</button>
        </nav>
      </header>
    );
  }

  return (
    <header className="nav">
      <Link to="/" className="brand">🛍️ <b>ShopGo</b></Link>
      <nav className="nav-links">
        <Link to="/">Shop</Link>
        {user && <Link to="/cart">🛒 Cart{cartCount > 0 && <span className="pill">{cartCount}</span>}</Link>}
        {user && <Link to="/orders">My orders</Link>}
        {user?.role === 'admin' && <Link to="/admin" className="admin-entry">Admin panel</Link>}
        {user ? (
          <>
            <span className="muted small">Hi, {user.name.split(' ')[0]}</span>
            <button onClick={logout}>Logout</button>
          </>
        ) : (
          <>
            <Link to="/login">Login</Link>
            <Link to="/register"><button className="primary">Sign up</button></Link>
          </>
        )}
      </nav>
    </header>
  );
}

function Guard({ children, admin }) {
  const { user, ready } = useAuth();
  if (!ready) return <p className="muted">Loading…</p>;
  if (!user) return <Navigate to="/login" replace />;
  if (admin && user.role !== 'admin') return <Navigate to="/" replace />;
  return children;
}

export default function App() {
  const { pathname } = useLocation();
  useEffect(() => { window.scrollTo(0, 0); }, [pathname]);
  return (
    <>
      <Nav />
      <main className="container">
        <Routes>
          <Route path="/" element={<Shop />} />
          <Route path="/product/:id" element={<Product />} />
          <Route path="/login" element={<AuthForm mode="login" />} />
          <Route path="/register" element={<AuthForm mode="register" />} />
          <Route path="/cart" element={<Guard><Cart /></Guard>} />
          <Route path="/orders" element={<Guard><Orders /></Guard>} />
          <Route path="/admin" element={<Guard admin><Admin /></Guard>} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
      <footer className="footer">© {new Date().getFullYear()} ShopGo · Built with Go, Gin, PostgreSQL & React</footer>
    </>
  );
}
