/* ════════════════════════════════════════════════════════════════
   OKF Bundles — Frontend
   ISIS4426 Desarrollo de Soluciones Cloud
   ════════════════════════════════════════════════════════════════ */

const API = '/api';
let authToken = localStorage.getItem('okf_token') || null;
let currentUser = JSON.parse(localStorage.getItem('okf_user') || 'null');
let pollingIntervals = {};
let jobsCache = [];
let selectedJobId = null;

// ─── Utilidades de API ───────────────────────────────────────────────────────

async function apiRequest(method, path, body = null, isFormData = false) {
  const headers = {};
  if (authToken) headers['Authorization'] = `Bearer ${authToken}`;
  if (!isFormData && body) headers['Content-Type'] = 'application/json';

  const opts = {
    method,
    headers,
    body: isFormData ? body : (body ? JSON.stringify(body) : null),
  };

  let res;
  try {
    res = await fetch(API + path, opts);
  } catch (networkErr) {
    const err = new Error('No se pudo conectar con el servidor');
    err.isNetworkError = true;
    throw err;
  }

  const data = res.headers.get('content-type')?.includes('json')
    ? await res.json()
    : null;

  if (!res.ok) {
    throw new Error(data?.error || `Error ${res.status}`);
  }
  return data;
}

// ─── Iconos (Bootstrap Icons, cargado por CDN en index.html) ─────────────────
// Los nombres cortos de la izquierda son el vocabulario interno de la app;
// solo este mapa sabe traducirlos a la clase real de Bootstrap Icons, así
// que el resto del código nunca queda acoplado a esa librería en concreto.

const BI_ICON = {
  upload: 'upload', download: 'download', clock: 'clock', spin: 'arrow-repeat',
  check: 'check-lg', warn: 'exclamation-triangle', x: 'x-lg', slash: 'slash-circle',
  retry: 'arrow-clockwise', refresh: 'arrow-repeat', logout: 'box-arrow-right',
  file: 'file-earmark-text', plug: 'plug', tray: 'inbox',
};

function icon(name, extraClass = '') {
  const bi = BI_ICON[name] || name;
  return `<i class="bi bi-${bi} icon ${extraClass}" aria-hidden="true"></i>`;
}

// ─── Toasts ───────────────────────────────────────────────────────────────────

const TOAST_ICONS = { success: 'check', error: 'x', info: 'clock', warning: 'warn' };

function toast(type, title, message = '', duration = 4500) {
  const container = document.getElementById('toast-container');
  const el = document.createElement('div');
  el.className = `toast toast-${type}`;
  el.innerHTML = `
    ${icon(TOAST_ICONS[type] || 'clock', 'toast-icon')}
    <div class="toast-content">
      <div class="toast-title">${escapeHtml(title)}</div>
      ${message ? `<div class="toast-message">${escapeHtml(message)}</div>` : ''}
    </div>
  `;
  container.appendChild(el);

  setTimeout(() => {
    el.classList.add('removing');
    setTimeout(() => el.remove(), 220);
  }, duration);
}

// ─── Navegación de vistas ─────────────────────────────────────────────────────

function showAuth() {
  document.getElementById('view-app').hidden = true;
  document.getElementById('view-auth').hidden = false;
}

function showApp() {
  document.getElementById('view-auth').hidden = true;
  document.getElementById('view-app').hidden = false;
  document.getElementById('navbar-username').textContent = currentUser?.username || '';
  loadJobs();
}

// ─── Auth: Tabs ──────────────────────────────────────────────────────────────

document.querySelectorAll('.tab').forEach(tab => {
  tab.addEventListener('click', () => {
    document.querySelectorAll('.tab').forEach(t => { t.classList.remove('active'); t.setAttribute('aria-selected', 'false'); });
    tab.classList.add('active');
    tab.setAttribute('aria-selected', 'true');
    const target = tab.dataset.tab;
    document.querySelectorAll('.auth-form').forEach(f => f.classList.remove('active'));
    document.getElementById(`form-${target}`)?.classList.add('active');
    document.querySelectorAll('.form-alert').forEach(e => { e.hidden = true; });
  });
});

// ─── Auth: Registro ──────────────────────────────────────────────────────────

document.getElementById('form-register').addEventListener('submit', async (e) => {
  e.preventDefault();
  const btn = document.getElementById('btn-register');
  const errEl = document.getElementById('register-error');
  const password = document.getElementById('reg-password').value;

  if (password.length < 8) {
    errEl.textContent = 'La contraseña debe tener al menos 8 caracteres.';
    errEl.hidden = false;
    return;
  }

  btn.disabled = true;
  btn.textContent = 'Creando cuenta…';
  errEl.hidden = true;

  try {
    const data = await apiRequest('POST', '/auth/register', {
      username: document.getElementById('reg-username').value.trim(),
      email: document.getElementById('reg-email').value.trim(),
      password,
    });

    authToken = data.token;
    currentUser = data.user;
    localStorage.setItem('okf_token', authToken);
    localStorage.setItem('okf_user', JSON.stringify(currentUser));

    toast('success', 'Cuenta creada', `Bienvenido, ${currentUser.username}`);
    showApp();
  } catch (err) {
    errEl.textContent = err.message;
    errEl.hidden = false;
  } finally {
    btn.disabled = false;
    btn.textContent = 'Crear cuenta';
  }
});

// ─── Auth: Login ─────────────────────────────────────────────────────────────

document.getElementById('form-login').addEventListener('submit', async (e) => {
  e.preventDefault();
  const btn = document.getElementById('btn-login');
  const errEl = document.getElementById('login-error');

  btn.disabled = true;
  btn.textContent = 'Ingresando…';
  errEl.hidden = true;

  try {
    const data = await apiRequest('POST', '/auth/login', {
      email: document.getElementById('login-email').value.trim(),
      password: document.getElementById('login-password').value,
    });

    authToken = data.token;
    currentUser = data.user;
    localStorage.setItem('okf_token', authToken);
    localStorage.setItem('okf_user', JSON.stringify(currentUser));

    toast('success', 'Sesión iniciada', currentUser.username);
    showApp();
  } catch (err) {
    errEl.textContent = err.message;
    errEl.hidden = false;
  } finally {
    btn.disabled = false;
    btn.textContent = 'Iniciar sesión';
  }
});

// ─── Logout ──────────────────────────────────────────────────────────────────

document.getElementById('btn-logout').addEventListener('click', () => {
  authToken = null;
  currentUser = null;
  selectedJobId = null;
  localStorage.removeItem('okf_token');
  localStorage.removeItem('okf_user');
  Object.values(pollingIntervals).forEach(clearInterval);
  pollingIntervals = {};
  toast('info', 'Sesión cerrada');
  showAuth();
});

// ─── Upload: Drag & Drop ─────────────────────────────────────────────────────

const uploadZone = document.getElementById('upload-zone');
const fileInput = document.getElementById('file-input');
const uploadErrorEl = document.getElementById('upload-error');

uploadZone.addEventListener('click', () => fileInput.click());
uploadZone.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); fileInput.click(); }
});

uploadZone.addEventListener('dragover', (e) => { e.preventDefault(); uploadZone.classList.add('is-dragging'); });
uploadZone.addEventListener('dragleave', () => uploadZone.classList.remove('is-dragging'));
uploadZone.addEventListener('drop', (e) => {
  e.preventDefault();
  uploadZone.classList.remove('is-dragging');
  const file = e.dataTransfer.files[0];
  if (file) handleFileUpload(file);
});

fileInput.addEventListener('change', (e) => {
  const file = e.target.files[0];
  if (file) handleFileUpload(file);
  e.target.value = '';
});

// ─── Upload: Manejo principal ─────────────────────────────────────────────────

const VALID_EXTS = ['.md', '.markdown', '.html', '.htm', '.txt', '.pdf', '.docx'];
const MAX_SIZE = 50 * 1024 * 1024;

async function handleFileUpload(file) {
  uploadErrorEl.hidden = true;

  const ext = '.' + file.name.split('.').pop().toLowerCase();
  if (!VALID_EXTS.includes(ext)) {
    uploadErrorEl.textContent = `Formato no soportado (${ext}). Usa: ${VALID_EXTS.join(', ')}`;
    uploadErrorEl.hidden = false;
    return;
  }
  if (file.size > MAX_SIZE) {
    uploadErrorEl.textContent = `El archivo pesa ${formatSize(file.size)}; el límite es 50 MB.`;
    uploadErrorEl.hidden = false;
    return;
  }

  const progressEl = document.getElementById('upload-progress');
  const progressBar = document.getElementById('progress-bar');
  const progressLabel = document.getElementById('progress-label');
  const progressJobId = document.getElementById('progress-job-id');

  uploadZone.classList.add('is-uploading');
  progressLabel.textContent = `Subiendo ${file.name}…`;
  progressJobId.textContent = '';
  progressBar.style.width = '30%';
  progressEl.hidden = false;

  try {
    const formData = new FormData();
    formData.append('file', file);
    progressBar.style.width = '65%';

    const data = await apiRequest('POST', '/documents', formData, true);

    progressBar.style.width = '100%';
    progressLabel.textContent = 'Documento recibido — procesando en segundo plano';
    progressJobId.textContent = `job ${data.job_id.slice(0, 8)}`;

    toast('success', 'Documento recibido', 'Puedes cerrar esta pestaña; el trabajo sigue procesándose.');

    startPolling(data.job_id);
    await loadJobs();
    selectJob(data.job_id);

    setTimeout(() => { progressEl.hidden = true; progressBar.style.width = '0'; }, 4000);
  } catch (err) {
    progressEl.hidden = true;
    progressBar.style.width = '0';
    uploadErrorEl.textContent = err.message;
    uploadErrorEl.hidden = false;
    toast('error', 'Error al subir', err.message);
  } finally {
    uploadZone.classList.remove('is-uploading');
  }
}

// ─── Jobs: Cargar lista ──────────────────────────────────────────────────────

async function loadJobs({ silent = false } = {}) {
  const loadingEl = document.getElementById('jobs-loading');
  const errorEl = document.getElementById('jobs-network-error');
  const emptyEl = document.getElementById('jobs-empty');
  const listEl = document.getElementById('jobs-list');

  if (!silent && jobsCache.length === 0) {
    loadingEl.hidden = false;
    errorEl.hidden = true;
    emptyEl.hidden = true;
    listEl.hidden = true;
  }

  try {
    const data = await apiRequest('GET', '/jobs');
    jobsCache = data.jobs || [];
    loadingEl.hidden = true;
    errorEl.hidden = true;

    if (jobsCache.length === 0) {
      emptyEl.hidden = false;
      listEl.hidden = true;
    } else {
      emptyEl.hidden = true;
      listEl.hidden = false;
      renderJobs();
    }

    loadMetrics();
    if (selectedJobId) renderDetail(jobsCache.find(j => j.id === selectedJobId) || null);
  } catch (err) {
    loadingEl.hidden = true;
    if (jobsCache.length === 0) {
      errorEl.hidden = false;
      listEl.hidden = true;
    } else if (!silent) {
      toast('error', 'Error de red', err.message);
    }
  }
}

async function loadMetrics() {
  const statsEl = document.getElementById('list-stats');
  try {
    const metrics = await apiRequest('GET', '/metrics');
    let total = 0, completed = 0, active = 0, failed = 0;
    for (const [status, count] of Object.entries(metrics)) {
      total += count;
      if (status === 'completed') completed += count;
      if (status === 'pending' || status === 'processing') active += count;
      if (status === 'failed' || status === 'canceled') failed += count;
    }
    statsEl.innerHTML = `<b>${total}</b> total · <b>${completed}</b> completados · <b>${active}</b> activos · <b>${failed}</b> fallidos`;
  } catch (err) {
    statsEl.textContent = '';
  }
}

function renderJobs() {
  const listEl = document.getElementById('jobs-list');
  listEl.innerHTML = jobsCache.map(jobCardHTML).join('');
}

// ─── Estado visual de un job ──────────────────────────────────────────────────

const STATUS_META = {
  pending:    { label: 'En cola',            icon: 'clock' },
  processing: { label: 'Procesando',         icon: 'spin'  },
  completed:  { label: 'Completado',         icon: 'check' },
  warned:     { label: 'Con advertencias',   icon: 'warn'  },
  failed:     { label: 'Fallido',            icon: 'x'     },
  canceled:   { label: 'Cancelado',          icon: 'slash' },
};

const FORMAT_LABELS = {
  markdown: 'Markdown', html: 'HTML', plaintext: 'Texto', pdf: 'PDF', docx: 'DOCX',
};

// El estado "warned" es derivado: completed + validation_status con advertencias.
function displayStatus(job) {
  if (job.status === 'completed' && job.validation_status === 'valid_with_warnings') return 'warned';
  return job.status;
}

function statusBadgeHTML(job) {
  const key = displayStatus(job);
  const meta = STATUS_META[key] || { label: key, icon: 'clock' };
  return `<span class="status-badge" data-status="${key}">${icon(meta.icon)}${meta.label}</span>`;
}

function jobCardHTML(job) {
  const dateStr = formatDate(job.created_at);
  const conceptsText = job.concept_count > 0 ? `${job.concept_count} concepto${job.concept_count !== 1 ? 's' : ''}` : '';
  const sizeText = job.bundle_size > 0 ? formatSize(job.bundle_size) : '';
  const meta = [FORMAT_LABELS[job.format] || job.format, dateStr, conceptsText, sizeText].filter(Boolean).join(' · ');

  const canCancel = job.status === 'pending' || job.status === 'processing';
  const canRetry = job.status === 'failed' || job.status === 'canceled';
  const canDownload = job.status === 'completed';

  const actions = [
    canDownload ? `<button class="btn btn-ghost" type="button" title="Descargar bundle" onclick="downloadBundle('${job.id}', event)">${icon('download')}</button>` : '',
    canRetry ? `<button class="btn btn-ghost" type="button" title="Reintentar" onclick="retryJob('${job.id}', event)">${icon('retry')}</button>` : '',
    canCancel ? `<button class="btn btn-ghost" type="button" title="Cancelar" onclick="cancelJob('${job.id}', event)">${icon('x')}</button>` : '',
  ].join('');

  return `
    <li class="job-card" id="job-${job.id}">
      <button type="button" class="job-card-select" onclick="selectJob('${job.id}')" aria-current="${job.id === selectedJobId ? 'true' : 'false'}">
        ${statusBadgeHTML(job)}
        <span class="job-card-main">
          <span class="job-card-name mono">${escapeHtml(job.original_filename || job.filename)}</span>
          <span class="job-card-meta">${escapeHtml(meta)}</span>
        </span>
      </button>
      <div class="job-card-actions">${actions}</div>
    </li>
  `;
}

// ─── Panel de detalle ─────────────────────────────────────────────────────────

function selectJob(jobId) {
  selectedJobId = jobId;
  document.querySelectorAll('.job-card').forEach(c => c.classList.toggle('is-selected', c.id === `job-${jobId}`));
  const job = jobsCache.find(j => j.id === jobId);
  renderDetail(job || null);
}

function renderDetail(job) {
  const empty = document.getElementById('detail-empty');
  const panel = document.getElementById('detail-panel');

  if (!job) { empty.hidden = false; panel.hidden = true; return; }
  empty.hidden = true;
  panel.hidden = false;

  const key = displayStatus(job);
  const meta = STATUS_META[key] || { label: key, icon: 'clock' };

  let messageHTML = '';
  let warningsHTML = '';
  let actionsHTML = '';

  if (job.status === 'pending' || job.status === 'processing') {
    messageHTML = `<div class="detail-message tone-info">${job.status === 'pending' ? 'En espera de un worker disponible.' : 'Un worker está convirtiendo el documento ahora mismo.'}</div>`;
    messageHTML += `<p class="detail-async-note">${icon('clock')} El procesamiento es asíncrono: puedes cerrar esta pestaña y el trabajo continúa. Vuelve más tarde a descargarlo.</p>`;
    if (job.status === 'pending' || job.status === 'processing') {
      actionsHTML = `<button class="btn btn-danger-ghost" type="button" onclick="cancelJob('${job.id}', event)">${icon('x')} Cancelar</button>`;
    }
  } else if (job.status === 'completed' && key === 'warned') {
    messageHTML = `<div class="detail-message tone-warn">Bundle válido, con advertencias — revisa el detalle antes de confiar ciegamente en el resultado.</div>`;
    if (job.validation_warnings && job.validation_warnings.length > 0) {
      warningsHTML = `
        <div class="paper-inset">
          <p class="paper-inset-label">log.md — advertencias (${job.validation_warnings.length})</p>
          <div class="warning-list mono">
            ${job.validation_warnings.map(w => `<div class="warning-item">${escapeHtml(w)}</div>`).join('')}
          </div>
        </div>`;
    }
    actionsHTML = `<button class="btn btn-primary" type="button" onclick="downloadBundle('${job.id}', event)">${icon('download')} Descargar bundle</button>`;
  } else if (job.status === 'completed') {
    messageHTML = `<div class="detail-message tone-ok">Bundle válido, sin advertencias.</div>`;
    actionsHTML = `<button class="btn btn-primary" type="button" onclick="downloadBundle('${job.id}', event)">${icon('download')} Descargar bundle</button>`;
  } else if (job.status === 'failed') {
    messageHTML = `<div class="detail-message tone-danger">${escapeHtml(job.error_message || 'El trabajo falló sin un mensaje específico.')}</div>`;
    actionsHTML = `<button class="btn btn-secondary" type="button" onclick="retryJob('${job.id}', event)">${icon('retry')} Reintentar</button>`;
  } else if (job.status === 'canceled') {
    messageHTML = `<div class="detail-message tone-info">Cancelado por el usuario.</div>`;
    actionsHTML = `<button class="btn btn-secondary" type="button" onclick="retryJob('${job.id}', event)">${icon('retry')} Reintentar</button>`;
  }

  panel.innerHTML = `
    <div class="detail-head">
      <div class="detail-name mono">${escapeHtml(job.original_filename || job.filename)}</div>
      <div class="detail-badges">
        ${statusBadgeHTML(job)}
        <span class="format-badge">${FORMAT_LABELS[job.format] || job.format}</span>
      </div>
    </div>
    <div class="detail-body">
      ${messageHTML}
      ${warningsHTML}
      <dl class="detail-facts">
        <div class="detail-row"><dt>Job ID</dt><dd class="mono">${job.id}</dd></div>
        <div class="detail-row"><dt>Creado</dt><dd>${formatDate(job.created_at, true)}</dd></div>
        ${job.completed_at ? `<div class="detail-row"><dt>Finalizado</dt><dd>${formatDate(job.completed_at, true)}</dd></div>` : ''}
        ${job.concept_count > 0 ? `<div class="detail-row"><dt>Conceptos</dt><dd class="mono">${job.concept_count}</dd></div>` : ''}
        ${job.bundle_size > 0 ? `<div class="detail-row"><dt>Tamaño</dt><dd class="mono">${formatSize(job.bundle_size)}</dd></div>` : ''}
      </dl>
    </div>
    ${actionsHTML ? `<div class="detail-actions">${actionsHTML}</div>` : ''}
  `;
}

// ─── Polling de estado ────────────────────────────────────────────────────────

function startPolling(jobId) {
  if (pollingIntervals[jobId]) return;

  pollingIntervals[jobId] = setInterval(async () => {
    try {
      const job = await apiRequest('GET', `/jobs/${jobId}`);
      const idx = jobsCache.findIndex(j => j.id === jobId);
      if (idx >= 0) jobsCache[idx] = job; else jobsCache.unshift(job);
      renderJobs();
      if (selectedJobId === jobId) renderDetail(job);
      loadMetrics();

      if (job.status === 'completed') {
        clearInterval(pollingIntervals[jobId]);
        delete pollingIntervals[jobId];
        const key = displayStatus(job);
        toast(key === 'warned' ? 'warning' : 'success',
          key === 'warned' ? 'Bundle listo, con advertencias' : '¡Bundle listo!',
          `${job.original_filename} — ${job.concept_count} concepto(s)`);
      } else if (job.status === 'failed') {
        clearInterval(pollingIntervals[jobId]);
        delete pollingIntervals[jobId];
        toast('error', 'Conversión fallida', job.error_message || 'Error desconocido');
      }
    } catch (err) {
      // red intermitente durante el polling: se reintenta en el próximo tick
    }
  }, 3000);
}

// ─── Cancelar, reintentar, descargar ──────────────────────────────────────────

async function cancelJob(jobId, evt) {
  evt?.stopPropagation();
  try {
    await apiRequest('DELETE', `/jobs/${jobId}`);
    toast('info', 'Trabajo cancelado');
    await loadJobs();
  } catch (err) {
    toast('error', 'Error al cancelar', err.message);
  }
}

async function retryJob(jobId, evt) {
  evt?.stopPropagation();
  try {
    const data = await apiRequest('POST', `/jobs/${jobId}/retry`);
    toast('success', 'Trabajo reintentado', data.message);
    startPolling(data.job_id);
    await loadJobs();
    selectJob(data.job_id);
  } catch (err) {
    toast('error', 'Error al reintentar', err.message);
  }
}

async function downloadBundle(jobId, evt) {
  evt?.stopPropagation();
  try {
    const res = await fetch(`${API}/jobs/${jobId}/download`, {
      headers: { 'Authorization': `Bearer ${authToken}` },
    });
    if (!res.ok) {
      const err = await res.json();
      throw new Error(err.error || `Error ${res.status}`);
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `bundle-${jobId.slice(0, 8)}.zip`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    toast('success', 'Descarga iniciada');
  } catch (err) {
    toast('error', 'Error al descargar', err.message);
  }
}

// ─── Refresh manual / reconexión ──────────────────────────────────────────────

document.getElementById('btn-refresh').addEventListener('click', () => {
  loadJobs();
  toast('info', 'Lista actualizada');
});
document.getElementById('btn-retry-connection').addEventListener('click', () => loadJobs());

// ─── Utilidades ──────────────────────────────────────────────────────────────

function formatSize(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDate(iso, withTime = false) {
  const d = new Date(iso);
  const opts = { day: '2-digit', month: '2-digit', year: 'numeric' };
  if (withTime) { opts.hour = '2-digit'; opts.minute = '2-digit'; }
  return d.toLocaleString('es-CO', opts);
}

function escapeHtml(text) {
  const div = document.createElement('div');
  div.appendChild(document.createTextNode(text ?? ''));
  return div.innerHTML;
}

// ─── Inicialización ───────────────────────────────────────────────────────────

function init() {
  if (authToken && currentUser) {
    showApp();
    apiRequest('GET', '/jobs').then(data => {
      (data.jobs || [])
        .filter(j => j.status === 'pending' || j.status === 'processing')
        .forEach(j => startPolling(j.id));
    }).catch(() => {});
  } else {
    showAuth();
  }
}

init();
