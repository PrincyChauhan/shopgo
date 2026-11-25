import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { api, money } from '../api';
import { useAuth } from '../auth.jsx';
import { useToast } from '../toast.jsx';
import { ProductArt } from '../ui.jsx';

export default function Shop() {
  const nav = useNavigate();
  const toast = useToast();
  const { user, refreshCart } = useAuth();
  const [cats, setCats] = useState([]);
  const [f, setF] = useState({ q: '', category_id: '', sort: 'newest', in_stock: false, page: 1 });
  const [res, setRes] = useState(null); // null = loading

  useEffect(() => { api('/categories').then(setCats).catch(() => {}); }, []);

  // Re-fetch whenever a filter changes (small delay so typing doesn't spam the server).
  useEffect(() => {
    const t = setTimeout(() => {
      const p = new URLSearchParams({ sort: f.sort, page: f.page, limit: 8 });
      if (f.q) p.set('q', f.q);
      if (f.category_id) p.set('category_id', f.category_id);
      if (f.in_stock) p.set('in_stock', 'true');
      api('/products?' + p).then(setRes).catch((e) => toast(e.message, 'err'));
    }, 250);
    return () => clearTimeout(t);
  }, [f]);

  const set = (k, v) => setF({ ...f, [k]: v, page: k === 'page' ? v : 1 });

  async function add(id) {
    if (!user) return nav('/login');
    try {
      await api('/cart/items', { method: 'POST', body: { product_id: id, quantity: 1 } });
      refreshCart();
      toast('Added to cart ✓');
    } catch (e) {
      toast(e.message, 'err');
    }
  }

  return (
    <>
      <section className="hero">
        <h1>Everything you need, delivered to your door.</h1>
        <p>Browse the catalog, pay securely with UPI, cards or netbanking, and track your order.</p>
        <div className="searchbar">
          <input placeholder="Search for products…" value={f.q} onChange={(e) => set('q', e.target.value)} />
        </div>
        <div className="perks"><span>🚚 Free delivery</span><span>🔒 Secure payments</span><span>↩️ Easy cancellation</span></div>
      </section>

      <div className="toolbar">
        <div className="chips">
          <button className={'chip' + (f.category_id === '' ? ' active' : '')} onClick={() => set('category_id', '')}>All</button>
          {cats.map((c) => (
            <button key={c.id} className={'chip' + (String(c.id) === f.category_id ? ' active' : '')} onClick={() => set('category_id', String(c.id))}>{c.name}</button>
          ))}
        </div>
        <div className="right">
          <label className="check"><input type="checkbox" checked={f.in_stock} onChange={(e) => set('in_stock', e.target.checked)} /> In stock</label>
          <select value={f.sort} onChange={(e) => set('sort', e.target.value)}>
            <option value="newest">Newest</option>
            <option value="price_asc">Price: low to high</option>
            <option value="price_desc">Price: high to low</option>
            <option value="name">Name</option>
          </select>
        </div>
      </div>

      {!res && <div className="grid">{[...Array(8)].map((_, i) => <div key={i} className="skeleton" />)}</div>}

      {res && (
        <>
          <div className="grid">
            {res.data.map((p) => (
              <div className="card" key={p.id}>
                <Link to={`/product/${p.id}`}><ProductArt product={p} /></Link>
                <div className="card-body">
                  <span className="cat">{p.category?.name}</span>
                  <Link to={`/product/${p.id}`} className="title">{p.name}</Link>
                  <div className="row">
                    <strong className="price">{money(p.price)}</strong>
                    {p.stock === 0 && <span className="tag bad">Out of stock</span>}
                    {p.stock > 0 && p.stock <= 5 && <span className="tag">Only {p.stock} left</span>}
                  </div>
                  <button className="primary" disabled={p.stock === 0} onClick={() => add(p.id)}>Add to cart</button>
                </div>
              </div>
            ))}
          </div>
          {res.data.length === 0 && <div className="empty"><div className="big">🔍</div><p className="muted">No products match your search.</p></div>}
          {res.total_pages > 1 && (
            <div className="pager">
              <button disabled={f.page <= 1} onClick={() => set('page', f.page - 1)}>← Prev</button>
              <span className="muted">Page {f.page} of {res.total_pages}</span>
              <button disabled={f.page >= res.total_pages} onClick={() => set('page', f.page + 1)}>Next →</button>
            </div>
          )}
        </>
      )}
    </>
  );
}
