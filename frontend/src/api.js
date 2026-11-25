// Prices arrive from the API in cents (paise). Convert only for display.
export const money = (cents) =>
  '₹' + (cents / 100).toLocaleString('en-IN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

// One wrapper for every request: adds the JWT, parses JSON, throws readable errors.
export async function api(path, { method = 'GET', body, form } = {}) {
  const token = localStorage.getItem('token');
  const headers = {};
  if (token) headers.Authorization = 'Bearer ' + token;
  if (body) headers['Content-Type'] = 'application/json';

  const res = await fetch('/api' + path, {
    method,
    headers,
    body: form ?? (body ? JSON.stringify(body) : undefined),
  });

  if (res.status === 204) return null;
  const data = await res.json().catch(() => ({}));

  if (!res.ok) {
    // Expired or invalid token: tell the app to log out.
    if (res.status === 401 && token && !path.startsWith('/auth/login')) {
      window.dispatchEvent(new Event('logout'));
    }
    throw new Error(data.error || 'Request failed');
  }
  return data;
}
