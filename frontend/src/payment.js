import { api } from './api';
import { openDemoPaymentModal } from './paymentModal.jsx';

function loadRazorpay() {
  return new Promise((resolve, reject) => {
    if (window.Razorpay) return resolve();
    const s = document.createElement('script');
    s.src = 'https://checkout.razorpay.com/v1/checkout.js';
    s.onload = resolve;
    s.onerror = () => reject(new Error('Could not load Razorpay. Check your internet connection.'));
    document.body.appendChild(s);
  });
}

// Pays for an order. Resolves 'paid' or 'cancelled' (customer closed the popup).
// Throws an Error if something goes wrong.
export async function payForOrder(order, user) {
  const info = await api(`/orders/${order.id}/payment`, { method: 'POST' });

  // No Razorpay keys on the server: show the demo payment checkout popup.
  if (info.demo) {
    return openDemoPaymentModal(order, info.amount);
  }

  await loadRazorpay();
  return new Promise((resolve, reject) => {
    let settled = false;
    const rz = new window.Razorpay({
      key: info.key_id,
      amount: info.amount,
      currency: info.currency,
      order_id: info.razorpay_order_id,
      name: 'ShopGo',
      description: `Order #${order.id}`,
      prefill: { name: order.ship_name || user.name, email: user.email, contact: order.ship_phone },
      theme: { color: '#5b5bf0' },
      // Called after a successful payment. The SERVER checks the signature
      // before marking the order paid; the browser is never trusted.
      handler: async (resp) => {
        settled = true;
        try {
          await api(`/orders/${order.id}/payment/verify`, { method: 'POST', body: resp });
          resolve('paid');
        } catch (e) {
          reject(e);
        }
      },
      modal: {
        ondismiss: () => {
          if (!settled) resolve('cancelled');
        },
      },
    });
    rz.on('payment.failed', (resp) => {
      if (!settled) {
        settled = true;
        reject(new Error(resp?.error?.description || 'Payment failed. Please try again.'));
      }
    });
    rz.open();
  });
}
