// Small shared pieces.
const ART = {
  Electronics: ['🎧', '#6366f1', '#8b5cf6'],
  Books: ['📚', '#0ea5e9', '#6366f1'],
  Clothing: ['👕', '#ec4899', '#f97316'],
  'Home & Kitchen': ['🍳', '#10b981', '#0ea5e9'],
};
const FALLBACK = ['🛍️', '#64748b', '#94a3b8'];

// Real photo if the admin uploaded one, otherwise a colourful placeholder.
export function ProductArt({ product, large }) {
  const cls = 'art' + (large ? ' large' : '');
  if (product.image_url) return <img className={cls} src={product.image_url} alt={product.name} loading="lazy" />;
  const [emoji, a, b] = ART[product.category?.name] || FALLBACK;
  return <div className={cls} style={{ background: `linear-gradient(135deg, ${a}, ${b})` }}><span>{emoji}</span></div>;
}

const FLOW = ['pending', 'paid', 'shipped', 'delivered'];
const LABEL = { pending: 'Awaiting payment' };

// Order progress tracker.
export function Steps({ status }) {
  if (status === 'cancelled') return <div className="steps"><span className="badge cancelled">Cancelled</span></div>;
  const idx = FLOW.indexOf(status);
  return (
    <div className="steps">
      {FLOW.map((s, i) => (
        <div key={s} className={'step' + (i <= idx && s !== 'pending' ? ' done' : '')}>
          <i>{i <= idx && s !== 'pending' ? '✓' : i + 1}</i><span>{LABEL[s] || s}</span>
        </div>
      ))}
    </div>
  );
}

export const Address = ({ o }) =>
  o.ship_name ? (
    <div className="addr">
      📍 {o.ship_name} · {o.ship_phone}<br />
      {o.ship_address}, {o.ship_city}, {o.ship_state} {o.ship_pincode}
    </div>
  ) : null;
