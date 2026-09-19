const BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

function authHeaders() {
  const token = localStorage.getItem('token');
  return token ? { Authorization: `Bearer ${token}` } : {};
}

async function request(path, options = {}) {
  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...authHeaders(),
      ...(options.headers || {}),
    },
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || `Request failed (${res.status})`);
  }
  return data;
}

export const api = {
  signup: (username, password) =>
    request('/api/signup', { method: 'POST', body: JSON.stringify({ username, password }) }),
  login: (username, password) =>
    request('/api/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
  createPoll: (question, options) =>
    request('/api/polls', { method: 'POST', body: JSON.stringify({ question, options }) }),
  getPoll: (id) => request(`/api/polls/${id}`),
  getResults: (id) => request(`/api/polls/${id}/results`),
  vote: (id, optionIdx, voterId) =>
    request(`/api/polls/${id}/vote`, { method: 'POST', body: JSON.stringify({ optionIdx, voterId }) }),
  myPolls: () => request('/api/my-polls'),
  closePoll: (id) => request(`/api/polls/${id}/close`, { method: 'POST' }),
  wsUrl: (id) => `${BASE_URL.replace(/^http/, 'ws')}/api/polls/${id}/watch`,
};

export function getOrCreateVoterId() {
  let id = localStorage.getItem('voterId');
  if (!id) {
    id = crypto.randomUUID();
    localStorage.setItem('voterId', id);
  }
  return id;
}
