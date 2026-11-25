import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { api } from '../api';
import { useAuth } from '../auth.jsx';

export default function AuthForm({ mode }) {
  const isRegister = mode === 'register';
  const nav = useNavigate();
  const { login } = useAuth();
  const [form, setForm] = useState({ name: '', email: '', password: '' });
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);

  const change = (k) => (e) => setForm({ ...form, [k]: e.target.value });

  async function submit(e) {
    e.preventDefault();
    setErr('');
    setBusy(true);
    try {
      const body = isRegister ? form : { email: form.email, password: form.password };
      const data = await api(isRegister ? '/auth/register' : '/auth/login', { method: 'POST', body });
      login(data.token, data.user);
      nav(data.user.role === 'admin' ? '/admin' : '/');
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="auth">
      <div className="auth-side">
        <h2>{isRegister ? 'Join ShopGo' : 'Welcome back 👋'}</h2>
        <p>Shop thousands of products, pay securely, and track every order in one place.</p>
        <p className="small">🔒 Passwords are stored encrypted.</p>
      </div>
      <form className="auth-form" onSubmit={submit}>
        <h2>{isRegister ? 'Create your account' : 'Login'}</h2>
        {err && <p className="error">{err}</p>}
        {isRegister && <label className="fl">Full name<input value={form.name} onChange={change('name')} required /></label>}
        <label className="fl">Email<input type="email" value={form.email} onChange={change('email')} required /></label>
        <label className="fl">Password<input type="password" placeholder={isRegister ? 'At least 8 characters' : ''} value={form.password} onChange={change('password')} required /></label>
        <button className="primary wide" disabled={busy}>{busy ? 'Please wait…' : isRegister ? 'Create account' : 'Login'}</button>
        <p className="small muted">
          {isRegister ? <>Already have an account? <Link to="/login">Login</Link></> : <>New here? <Link to="/register">Create an account</Link></>}
        </p>
      </form>
    </div>
  );
}
