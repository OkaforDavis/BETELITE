// HTTP client for the Go API. Attaches the Firebase ID token automatically.
const host = location.hostname;
export const API_BASE =
  host.endsWith('github.io') ? 'https://betelite-alvn.onrender.com' : '';

let tokenProvider = async () => null;
export function setTokenProvider(fn) { tokenProvider = fn; }
export const currentToken = () => tokenProvider();

export class ApiError extends Error {
  constructor(status, message) {
    super(message);
    this.status = status;
  }
}

export async function api(method, path, body, { form = false, raw = false } = {}) {
  const headers = {};
  const token = await tokenProvider();
  if (token) headers.Authorization = 'Bearer ' + token;
  let payload;
  if (body !== undefined) {
    if (form) payload = body; // FormData: browser sets the boundary
    else { headers['Content-Type'] = 'application/json'; payload = JSON.stringify(body); }
  }
  let res;
  try {
    res = await fetch(API_BASE + '/api' + path, { method, headers, body: payload });
  } catch {
    throw new ApiError(0, "You're offline. Check your connection and try again.");
  }
  if (raw) return res;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(res.status, data.error || 'Something went wrong. Please try again.');
  return data;
}

export const get = (p) => api('GET', p);
export const post = (p, b = {}) => api('POST', p, b);
export const put = (p, b = {}) => api('PUT', p, b);
export const del = (p) => api('DELETE', p);
export const upload = (p, formData) => api('POST', p, formData, { form: true });
