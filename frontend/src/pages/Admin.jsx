import { useEffect, useState } from 'react';
import { api, money } from '../api';
import { useToast } from '../toast.jsx';
import { Address, ProductArt } from '../ui.jsx';

const NEXT = { pending: ['paid', 'cancelled'], paid: ['shipped', 'cancelled'], shipped: ['delivered'] };
const emptyForm = { name: '', description: '', price: '', stock: '0', category_id: '' };

const TABS = [
  ['dashboard', 'Dashboard', 'Revenue, customers, and stock that needs attention'],
  ['products', 'Catalog', 'Categories, product details, prices, and photos'],
  ['orders', 'Orders', 'Shipping addresses and the path from paid to delivered'],
];

export default function Admin() {
  const [tab, setTab] = useState('dashboard');
  const current = TABS.find(([k]) => k === tab);
  return (
    <div className="admin-shell">
      <aside className="admin-side">
        <p className="admin-kicker">Store admin</p>
        {TABS.map(([k, label]) => (
          <button key={k} className={tab === k ? 'active' : ''} onClick={() => setTab(k)}>{label}</button>
        ))}
      </aside>
      <section className="admin-main">
        <header className="admin-head">
          <h2>{current[1]}</h2>
          <p className="muted">{current[2]}</p>
        </header>
        {tab === 'dashboard' && <Dashboard />}
        {tab === 'products' && <Products />}
        {tab === 'orders' && <AllOrders />}
      </section>
    </div>
  );
}

function Dashboard() {
  const toast = useToast();
  const [s, setS] = useState(null);
  useEffect(() => { api('/admin/stats').then(setS).catch((e) => toast(e.message, 'err')); }, []);
  if (!s) return <div className="skeleton" style={{ height: 160 }} />;

  const by = s.orders_by_status || {};
  return (
    <>
      <div className="stats">
        <div className="stat"><span>Revenue (paid orders)</span><strong>{money(s.revenue)}</strong></div>
        <div className="stat"><span>Total orders</span><strong>{s.orders_total}</strong></div>
        <div className="stat"><span>Customers</span><strong>{s.customers}</strong></div>
        <div className="stat"><span>Products</span><strong>{s.products}</strong></div>
      </div>
      <div className="panel">
        <h3>Orders by status</h3>
        <div className="chips">
          {['pending', 'paid', 'shipped', 'delivered', 'cancelled'].map((k) => (
            <span key={k} className={'badge ' + k}>{k}: {by[k] || 0}</span>
          ))}
        </div>
      </div>
      <div className="panel">
        <h3>Low stock (5 or fewer)</h3>
        {s.low_stock.length === 0 && <p className="muted small">All products are well stocked 👍</p>}
        {s.low_stock.map((p) => (
          <div className="line" key={p.id}>
            <span className="grow">{p.name}</span>
            <span className={p.stock === 0 ? 'bad' : 'ok'}>{p.stock === 0 ? 'Out of stock' : `${p.stock} left`}</span>
          </div>
        ))}
      </div>
    </>
  );
}

function Products() {
  const toast = useToast();
  const [products, setProducts] = useState([]);
  const [cats, setCats] = useState([]);
  const [form, setForm] = useState(emptyForm);
  const [editId, setEditId] = useState(null);
  const [file, setFile] = useState(null);
  const [newCat, setNewCat] = useState('');

  const load = () => {
    api('/products?limit=50').then((r) => setProducts(r.data)).catch((e) => toast(e.message, 'err'));
    api('/categories').then(setCats).catch(() => {});
  };
  useEffect(load, []);

  const change = (k) => (e) => setForm({ ...form, [k]: e.target.value });
  const reset = () => { setForm(emptyForm); setEditId(null); setFile(null); };

  async function save(e) {
    e.preventDefault();
    try {
      const body = {
        name: form.name,
        description: form.description,
        price: Math.round(parseFloat(form.price) * 100), // rupees → paise
        stock: parseInt(form.stock, 10),
        category_id: Number(form.category_id),
      };
      const saved = editId
        ? await api(`/admin/products/${editId}`, { method: 'PUT', body })
        : await api('/admin/products', { method: 'POST', body });
      if (file) {
        const fd = new FormData();
        fd.append('image', file);
        await api(`/admin/products/${saved.id}/image`, { method: 'POST', form: fd });
      }
      toast(editId ? 'Product updated ✓' : 'Product created ✓');
      reset();
      load();
    } catch (e2) {
      toast(e2.message, 'err');
    }
  }

  const edit = (p) => {
    setEditId(p.id);
    setFile(null);
    setForm({ name: p.name, description: p.description, price: String(p.price / 100), stock: String(p.stock), category_id: String(p.category_id) });
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };
  const del = async (p) => {
    if (!confirm(`Delete "${p.name}"?`)) return;
    try { await api(`/admin/products/${p.id}`, { method: 'DELETE' }); toast('Deleted', 'info'); load(); } catch (e) { toast(e.message, 'err'); }
  };
  const addCat = async (e) => {
    e.preventDefault();
    try { await api('/admin/categories', { method: 'POST', body: { name: newCat } }); setNewCat(''); load(); } catch (e2) { toast(e2.message, 'err'); }
  };
  const delCat = async (c) => {
    try { await api(`/admin/categories/${c.id}`, { method: 'DELETE' }); load(); } catch (e) { toast(e.message, 'err'); }
  };

  return (
    <>
      <div className="panel">
        <h3>Categories</h3>
        <div className="chips">
          {cats.map((c) => <span className="chip static" key={c.id}>{c.name} <button className="link bad" onClick={() => delCat(c)}>×</button></span>)}
        </div>
        <form className="inline" onSubmit={addCat}>
          <input placeholder="New category name" value={newCat} onChange={(e) => setNewCat(e.target.value)} required />
          <button>Add</button>
        </form>
      </div>

      <form className="panel" onSubmit={save}>
        <h3>{editId ? `Edit product #${editId}` : 'Add product'}</h3>
        <div className="fgrid">
          <label className="fl full">Name<input value={form.name} onChange={change('name')} required /></label>
          <label className="fl full">Description<textarea value={form.description} onChange={change('description')} /></label>
          <label className="fl">Price (₹)<input type="number" step="0.01" min="0.01" value={form.price} onChange={change('price')} required /></label>
          <label className="fl">Stock<input type="number" min="0" value={form.stock} onChange={change('stock')} required /></label>
          <label className="fl">Category
            <select value={form.category_id} onChange={change('category_id')} required>
              <option value="">Choose…</option>
              {cats.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
            </select>
          </label>
          <label className="fl">Image (jpg, png, gif, webp · max 5MB)<input type="file" accept="image/*" onChange={(e) => setFile(e.target.files[0] || null)} /></label>
        </div>
        <div className="inline">
          <button className="primary">{editId ? 'Save changes' : 'Create product'}</button>
          {editId && <button type="button" onClick={reset}>Cancel</button>}
        </div>
      </form>

      <div className="panel">
        <h3>Products ({products.length})</h3>
        {products.map((p) => (
          <div className="line" key={p.id}>
            <div className="thumb"><ProductArt product={p} /></div>
            <span className="grow"><strong>{p.name}</strong><br /><span className="small muted">{p.category?.name}</span></span>
            <span>{money(p.price)}</span>
            <span className={p.stock > 0 ? 'ok' : 'bad'}>{p.stock} in stock</span>
            <button onClick={() => edit(p)}>Edit</button>
            <button className="danger" onClick={() => del(p)}>Delete</button>
          </div>
        ))}
      </div>
    </>
  );
}

function AllOrders() {
  const toast = useToast();
  const [orders, setOrders] = useState([]);
  const [status, setStatus] = useState('');

  const load = () => {
    const q = status ? `&status=${status}` : '';
    api(`/admin/orders?limit=50${q}`).then((r) => setOrders(r.data)).catch((e) => toast(e.message, 'err'));
  };
  useEffect(load, [status]);

  const move = async (id, to) => {
    try { await api(`/admin/orders/${id}/status`, { method: 'PUT', body: { status: to } }); toast(`Order #${id} marked ${to}`); load(); }
    catch (e) { toast(e.message, 'err'); }
  };

  return (
    <>
      <div className="toolbar">
        <select style={{ width: 'auto' }} value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">All statuses</option>
          {['pending', 'paid', 'shipped', 'delivered', 'cancelled'].map((s) => <option key={s}>{s}</option>)}
        </select>
      </div>
      {orders.length === 0 && <p className="muted">No orders.</p>}
      {orders.map((o) => (
        <div className="panel" key={o.id}>
          <div className="row">
            <strong>#{o.id} · {o.customer_name} <span className="small muted">{o.customer_email}</span></strong>
            <span className={'badge ' + o.status}>{o.status}</span>
          </div>
          {o.items.map((i) => <div className="small" key={i.product_id}>{i.name} × {i.quantity} — {money(i.price * i.quantity)}</div>)}
          <Address o={o} />
          <div className="row">
            <strong>{money(o.total)}</strong>
            <span className="inline">
              {(NEXT[o.status] || []).map((s) => (
                <button key={s} className={s === 'cancelled' ? 'danger' : 'primary'} onClick={() => move(o.id, s)}>Mark {s}</button>
              ))}
            </span>
          </div>
        </div>
      ))}
    </>
  );
}
