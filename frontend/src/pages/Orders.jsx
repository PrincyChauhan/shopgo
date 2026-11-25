import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api, money } from '../api';
import { useAuth } from '../auth.jsx';
import { useToast } from '../toast.jsx';
import { payForOrder } from '../payment.js';
import { Address, Steps } from '../ui.jsx';

export default function Orders() {
  const { user } = useAuth();
  const toast = useToast();
  const [orders, setOrders] = useState(null);
  const [busyId, setBusyId] = useState(null);

  const load = () => api('/orders?limit=50').then((r) => setOrders(r.data)).catch((e) => toast(e.message, 'err'));
  useEffect(() => { load(); }, []);

  async function pay(o) {
    setBusyId(o.id);
    try {
      const r = await payForOrder(o, user);
      if (r === 'paid') toast('Payment successful 🎉'); else toast('Payment cancelled', 'info');
    } catch (e) {
      toast(e.message, 'err');
    } finally {
      setBusyId(null);
      load();
    }
  }

  async function cancel(o) {
    if (!confirm(`Cancel order #${o.id}?`)) return;
    try { await api(`/orders/${o.id}/cancel`, { method: 'POST' }); toast('Order cancelled', 'info'); } catch (e) { toast(e.message, 'err'); }
    load();
  }

  if (!orders) return <div className="skeleton" style={{ height: 200 }} />;
  if (orders.length === 0) {
    return <div className="panel empty"><div className="big">📦</div><h2>No orders yet</h2><div><Link to="/"><button className="primary">Start shopping</button></Link></div></div>;
  }

  return (
    <>
      <h2 style={{ marginBottom: 16 }}>My orders</h2>
      {orders.map((o) => (
        <div className="panel" key={o.id}>
          <div className="row">
            <div><strong>Order #{o.id}</strong> <span className="small muted">· {new Date(o.created_at).toLocaleString()}</span></div>
            <span className={'badge ' + o.status}>{o.status}</span>
          </div>
          <Steps status={o.status} />
          <div>
            {o.items.map((i) => (
              <div className="line" key={i.product_id}>
                <span className="grow">{i.name} <span className="muted">× {i.quantity}</span></span>
                <span className="amount">{money(i.price * i.quantity)}</span>
              </div>
            ))}
          </div>
          <Address o={o} />
          <div className="row">
            <strong>Total {money(o.total)}</strong>
            <span className="inline">
              {o.status === 'pending' && <button className="primary" disabled={busyId === o.id} onClick={() => pay(o)}>{busyId === o.id ? 'Opening…' : 'Pay now'}</button>}
              {(o.status === 'pending' || o.status === 'paid') && <button className="danger" onClick={() => cancel(o)}>Cancel order</button>}
            </span>
          </div>
        </div>
      ))}
    </>
  );
}
