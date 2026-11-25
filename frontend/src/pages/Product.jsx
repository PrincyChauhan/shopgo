import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { api, money } from '../api';
import { useAuth } from '../auth.jsx';
import { useToast } from '../toast.jsx';
import { ProductArt } from '../ui.jsx';

export default function Product() {
  const { id } = useParams();
  const nav = useNavigate();
  const toast = useToast();
  const { user, refreshCart } = useAuth();
  const [p, setP] = useState(null);
  const [qty, setQty] = useState(1);
  const [err, setErr] = useState('');

  useEffect(() => {
    setP(null);
    api('/products/' + id).then(setP).catch((e) => setErr(e.message));
  }, [id]);

  async function add(goToCart) {
    if (!user) return nav('/login');
    try {
      await api('/cart/items', { method: 'POST', body: { product_id: p.id, quantity: qty } });
      refreshCart();
      if (goToCart) nav('/cart'); else toast('Added to cart ✓');
    } catch (e) {
      toast(e.message, 'err');
    }
  }

  if (err) return <div className="empty"><div className="big">😕</div><p>{err}</p><Link to="/">← Back to shop</Link></div>;
  if (!p) return <div className="skeleton" style={{ height: 380 }} />;

  const max = Math.min(p.stock, 10);
  return (
    <>
      <p className="small"><Link to="/">← Back to shop</Link></p>
      <div className="detail">
        <ProductArt product={p} large />
        <div className="detail-info">
          <span className="cat">{p.category?.name}</span>
          <h1>{p.name}</h1>
          <div className="big-price">{money(p.price)}</div>
          <p className="muted">{p.description || 'No description yet.'}</p>
          {p.stock === 0 && <span className="tag bad" style={{ alignSelf: 'flex-start' }}>Out of stock</span>}
          {p.stock > 0 && p.stock <= 5 && <span className="tag" style={{ alignSelf: 'flex-start' }}>Only {p.stock} left</span>}
          {p.stock > 0 && (
            <>
              <label className="fl" style={{ maxWidth: 120 }}>Quantity
                <select value={qty} onChange={(e) => setQty(Number(e.target.value))}>
                  {[...Array(max)].map((_, i) => <option key={i + 1}>{i + 1}</option>)}
                </select>
              </label>
              <div className="inline" style={{ maxWidth: 420 }}>
                <button onClick={() => add(false)}>Add to cart</button>
                <button className="primary" onClick={() => add(true)}>Buy now</button>
              </div>
            </>
          )}
          <p className="small muted">🚚 Free delivery · 🔒 Secure payment · ↩️ Cancel before shipping</p>
        </div>
      </div>
    </>
  );
}
