// api.js - every call to the Go server, in one place.
//
// The server holds the bkn token; this file only ever handles the session
// cookie the browser attaches by itself. Nothing here should ever learn what a
// bearer token looks like.

async function apiCall(path, options = {}) {
  const res = await fetch(path, {
    credentials: 'same-origin',
    ...options,
  });

  let body = null;
  try { body = await res.json(); } catch (e) { body = null; }

  if (!res.ok) {
    const err = new Error((body && body.error && body.error.message) || `request failed (${res.status})`);
    err.type = body && body.error && body.error.type;
    err.status = res.status;
    throw err;
  }
  return body;
}

const api = {
  me:     ()             => apiCall('/api/me'),
  login:  (email, password, remember) => apiCall('/api/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password, remember: !!remember }),
          }),
  logout: ()             => apiCall('/api/logout', { method: 'POST' }),

  drive:  (input)        => apiCall('/api/drive', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(input),
          }).then(r => r.value),

  // Upload reports progress, so it uses XHR: fetch cannot report how far a
  // request body has got, and a 25MB upload with no feedback looks broken.
  upload: (drive, path, file, onProgress) => new Promise((resolve, reject) => {
            const form = new FormData();
            form.append('drive', drive);
            form.append('path', path);
            form.append('file', file);

            const xhr = new XMLHttpRequest();
            xhr.open('POST', '/api/upload');
            xhr.upload.onprogress = (e) => {
              if (e.lengthComputable && onProgress) onProgress(e.loaded / e.total);
            };
            xhr.onload = () => {
              let body = null;
              try { body = JSON.parse(xhr.responseText); } catch (e) { /* keep null */ }
              if (xhr.status >= 200 && xhr.status < 300) return resolve(body);
              const msg = (body && (body.error?.message || body.error)) || `upload failed (${xhr.status})`;
              const err = new Error(msg);
              err.status = xhr.status;
              reject(err);
            };
            xhr.onerror = () => reject(new Error('the connection dropped during the upload'));
            xhr.send(form);
          }),

  downloadURL: (drive, path) =>
    `/api/download?drive=${encodeURIComponent(drive)}&path=${encodeURIComponent(path)}`,
};

function humanBytes(n) {
  if (n === null || n === undefined) return '—';
  n = Number(n);
  if (!isFinite(n)) return '—';
  if (n < 1024) return `${n} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let i = -1;
  do { n /= 1024; i++; } while (n >= 1024 && i < units.length - 1);
  return `${n < 10 ? n.toFixed(1) : Math.round(n)} ${units[i]}`;
}

function whenText(iso) {
  if (!iso) return '—';
  const d = new Date(iso);
  if (isNaN(d)) return '—';
  const mins = Math.round((Date.now() - d.getTime()) / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins} min ago`;
  const hours = Math.round(mins / 60);
  if (hours < 24) return `${hours} h ago`;
  const days = Math.round(hours / 24);
  if (days < 30) return `${days} d ago`;
  return d.toLocaleDateString();
}

function joinPath(parent, name) {
  return parent === '/' ? `/${name}` : `${parent}/${name}`;
}

function parentOf(path) {
  if (path === '/' || path === '') return '/';
  const at = path.lastIndexOf('/');
  return at <= 0 ? '/' : path.slice(0, at);
}
