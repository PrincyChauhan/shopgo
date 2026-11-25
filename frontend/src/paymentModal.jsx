import { useEffect, useState } from 'react';
import { api, money } from './api';

let openHandler = null;

export function openDemoPaymentModal(order, amount) {
  if (!openHandler) {
    return Promise.reject(new Error('Payment dialog is not ready'));
  }
  return openHandler(order, amount);
}

const BANKS = ['HDFC Bank', 'ICICI Bank', 'State Bank of India', 'Axis Bank', 'Kotak Mahindra Bank'];

export function PaymentModalHost() {
  const [req, setReq] = useState(null);
  const [method, setMethod] = useState('upi');
  const [upiId, setUpiId] = useState('success@razorpay');
  const [cardNum, setCardNum] = useState('');
  const [expiry, setExpiry] = useState('');
  const [cvv, setCvv] = useState('');
  const [bank, setBank] = useState(BANKS[0]);
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    openHandler = (order, amount) =>
      new Promise((resolve, reject) => {
        setMethod('upi');
        setUpiId('success@razorpay');
        setCardNum('');
        setExpiry('');
        setCvv('');
        setBank(BANKS[0]);
        setErr('');
        setBusy(false);
        setReq({ order, amount: amount ?? order.total, resolve, reject });
      });
    return () => {
      openHandler = null;
    };
  }, []);

  if (!req) return null;

  function cancel() {
    if (busy) return;
    const r = req;
    setReq(null);
    r.resolve('cancelled');
  }

  function validate() {
    if (method === 'upi') {
      if (!/^[\w.-]+@[\w.-]+$/.test(upiId.trim())) return 'Enter a valid UPI ID (e.g. name@upi)';
    } else if (method === 'card') {
      const digits = cardNum.replace(/\s+/g, '');
      if (!/^\d{12,19}$/.test(digits)) return 'Enter a valid 12–19 digit card number';
      if (!/^(0[1-9]|1[0-2])\/\d{2}$/.test(expiry.trim())) return 'Enter expiry as MM/YY';
      if (!/^\d{3,4}$/.test(cvv.trim())) return 'Enter a 3 or 4 digit CVV';
    }
    return '';
  }

  async function submit(e) {
    e.preventDefault();
    const vErr = validate();
    if (vErr) {
      setErr(vErr);
      return;
    }
    setErr('');
    setBusy(true);
    try {
      await api(`/orders/${req.order.id}/pay`, { method: 'POST', body: { method } });
      const r = req;
      setReq(null);
      r.resolve('paid');
    } catch (e2) {
      setErr(e2.message);
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="pay-title">
      <form className="modal" onSubmit={submit}>
        <div className="modal-head">
          <div>
            <span className="demo-pill">Demo checkout</span>
            <h3 id="pay-title">Pay for Order #{req.order.id}</h3>
          </div>
          <button type="button" className="link" onClick={cancel} disabled={busy} aria-label="Close">✕</button>
        </div>

        <div className="pay-amount">
          <span className="muted small">Amount payable</span>
          <strong>{money(req.amount)}</strong>
        </div>

        <div className="pay-tabs" role="tablist">
          {[
            ['upi', 'UPI'],
            ['card', 'Card'],
            ['netbanking', 'Netbanking'],
          ].map(([k, label]) => (
            <button
              key={k}
              type="button"
              className={'chip' + (method === k ? ' active' : '')}
              onClick={() => { setMethod(k); setErr(''); }}
            >
              {label}
            </button>
          ))}
        </div>

        {err && <p className="error">{err}</p>}

        {method === 'upi' && (
          <label className="fl">
            UPI ID
            <input value={upiId} onChange={(e) => setUpiId(e.target.value)} placeholder="name@upi" required />
            <span className="small muted">Demo mode: no real money is charged.</span>
          </label>
        )}

        {method === 'card' && (
          <div className="fgrid">
            <label className="fl full">
              Card number
              <input
                inputMode="numeric"
                placeholder="4111 1111 1111 1111"
                value={cardNum}
                onChange={(e) => setCardNum(e.target.value)}
                required
              />
            </label>
            <label className="fl">
              Expiry (MM/YY)
              <input placeholder="12/28" maxLength={5} value={expiry} onChange={(e) => setExpiry(e.target.value)} required />
            </label>
            <label className="fl">
              CVV
              <input type="password" inputMode="numeric" maxLength={4} placeholder="123" value={cvv} onChange={(e) => setCvv(e.target.value)} required />
            </label>
          </div>
        )}

        {method === 'netbanking' && (
          <label className="fl">
            Select bank
            <select value={bank} onChange={(e) => setBank(e.target.value)}>
              {BANKS.map((b) => <option key={b} value={b}>{b}</option>)}
            </select>
          </label>
        )}

        <div className="inline" style={{ marginTop: 4 }}>
          <button type="button" onClick={cancel} disabled={busy}>Pay later</button>
          <button type="submit" className="primary" disabled={busy}>
            {busy ? 'Processing…' : `Pay ${money(req.amount)}`}
          </button>
        </div>
      </form>
    </div>
  );
}
