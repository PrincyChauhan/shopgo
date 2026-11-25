import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { api, money } from '../api';
import { useAuth } from '../auth.jsx';
import { useToast } from '../toast.jsx';
import { payForOrder } from '../payment.js';
import { ProductArt } from '../ui.jsx';

const blank = { name: '', phone: '', address: '', city: '', state: '', pincode: '' };

export default function Cart() {
  const nav = useNavigate();
  const toast = useToast();
  const { user, refreshCart } = useAuth();
  const [cart, setCart] = useState(null);
  const [busy, setBusy] = useState(false);
  const [addr, setAddr] = useState(() => {
    try { return { ...blank, name: user?.name || '', ...JSON.parse(localStorage.getItem('address') || '{}') }; }
    catch { return { ...blank, name: user?.name || '' }; }
  });

  const load = () => api('/cart').then((c) => { setCart(c); refreshCart(); }).catch((e) => toast(e.message, 'err'));
  useEffect(() => { load(); }, []);

  const run = async (fn) => {
    try { await fn(); await load(); } catch (e) { toast(e.message, 'err'); }
  };
  const setQty = (id, quantity) => run(() => api(`/cart/items/${id}`, { method: 'PUT', body: { quantity } }));
  const remove = (id) => run(() => api(`/cart/items/${id}`, { method: 'DELETE' }));
  const field = (k) => ({ value: addr[k], onChange: (e) => setAddr({ ...addr, [k]: e.target.value }), required: true });

  // 1) create the order (checks stock, saves the address)  2) open the payment popup right away
  async function placeOrder(e) {
    e.preventDefault();
    setBusy(true);
    try {
      const order = await api('/orders', { method: 'POST', body: addr });
      localStorage.setItem('address', JSON.stringify(addr));
      refreshCart();
      try {
        const result = await payForOrder(order, user);
        if (result === 'paid') toast('Payment successful 🎉 Thank you!');
        else toast('Order saved. You can pay any time from My orders.', 'info');
      } catch (pe) {
        toast(pe.message, 'err');
      }
      nav('/orders');
    } catch (e2) {
      toast(e2.message, 'err');
      load();
    } finally {
      setBusy(false);
    }
  }

  if (!cart) return <div className="skeleton" style={{ height: 300 }} />;
  if (cart.items.length === 0) {
    return <div className="panel empty"><div className="big">🛒</div><h2>Your cart is empty</h2><p className="muted">Add something you like.</p><div><Link to="/"><button className="primary">Browse products</button></Link></div></div>;
  }

  return (
    <>
      <h2 style={{ marginBottom: 16 }}>Checkout</h2>
      <div className="cols">
        <div>
          <div className="panel">
            <h3>Your items ({cart.item_count})</h3>
            {cart.items.map((it) => (
              <div className="line" key={it.product_id}>
                <div className="thumb"><ProductArt product={it.product} /></div>
                <div className="grow">
                  <Link to={`/product/${it.product_id}`} className="title">{it.product.name}</Link>
                  <div className="small muted">{money(it.product.price)} each</div>
                </div>
                <div className="qty">
                  <button disabled={it.quantity <= 1} onClick={() => setQty(it.product_id, it.quantity - 1)}>−</button>
                  <span>{it.quantity}</span>
                  <button onClick={() => setQty(it.product_id, it.quantity + 1)}>+</button>
                </div>
                <span className="amount">{money(it.product.price * it.quantity)}</span>
                <button className="link bad" onClick={() => remove(it.product_id)}>Remove</button>
              </div>
            ))}
          </div>

          <form id="checkout" className="panel" onSubmit={placeOrder}>
            <h3>Delivery address</h3>
            <div className="fgrid">
              <label className="fl">Full name<input {...field('name')} /></label>
              <label className="fl">Phone<input type="tel" placeholder="10-digit mobile" {...field('phone')} /></label>
              <label className="fl full">Address<input placeholder="House no, street, area" {...field('address')} /></label>
              <label className="fl">City<input {...field('city')} /></label>
              <label className="fl">State<input {...field('state')} /></label>
              <label className="fl">Pincode<input inputMode="numeric" maxLength={6} placeholder="6 digits" {...field('pincode')} /></label>
            </div>
          </form>
        </div>

        <div className="panel summary">
          <h3>Order summary</h3>
          <div className="sum-row"><span className="muted">Subtotal</span><span>{money(cart.total)}</span></div>
          <div className="sum-row"><span className="muted">Delivery</span><span className="ok">Free</span></div>
          <div className="sum-row sum-total"><span>Total</span><span>{money(cart.total)}</span></div>
          <button form="checkout" className="primary wide" disabled={busy}>{busy ? 'Please wait…' : 'Place order & pay'}</button>
          <p className="small muted">🔒 Pay by UPI, card or netbanking. Stock is reserved when you place the order.</p>
        </div>
      </div>
    </>
  );
}
