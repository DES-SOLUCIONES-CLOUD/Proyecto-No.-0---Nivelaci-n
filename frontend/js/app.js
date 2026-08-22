/* ════════════════════════════════════════════════════════════════
   OKF Platform — Frontend JavaScript
   ISIS4426 Desarrollo de Soluciones Cloud
   ════════════════════════════════════════════════════════════════ */

const API = '/api';
let authToken = localStorage.getItem('okf_token') || null;
let currentUser = JSON.parse(localStorage.getItem('okf_user') || 'null');
let pollingIntervals = {};

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

  const res = await fetch(API + path, opts);
  const data = res.headers.get('content-type')?.includes('json')
    ? await res.json()
    : null;

  if (!res.ok) {
    throw new Error(data?.error || `Error ${res.status}`);
  }
  return data;
}

// ─── Toast Notifications ─────────────────────────────────────────────────────

function toast(type, title, message = '', duration = 4000) {
  const icons = { success: '✅', error: '❌', info: 'ℹ️', warning: '⚠️' };
  const container = document.getElementById('toast-container');

  const el = document.createElement('div');
  el.className = `toast toast-${type}`;
  el.innerHTML = `
    <span class="toast-icon">${icons[type]}</span>
    <div class="toast-content">
      <div class="toast-title">${title}</div>
      ${message ? `<div class="toast-message">${message}</div>` : ''}
    </div>
  `;
  container.appendChild(el);

  setTimeout(() => {
    el.classList.add('removing');
    setTimeout(() => el.remove(), 300);
  }, duration);
}

// ─── Navegación de páginas ───────────────────────────────────────────────────

function showPage(pageId) {
  document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
  document.getElementById(pageId)?.classList.add('active');
}

function showAuth() {
  document.getElementById('navbar').style.display = 'none';
  showPage('page-auth');
}

function showDashboard() {
  document.getElementById('navbar').style.display = 'flex';
  document.getElementById('navbar-username').textContent = currentUser?.username || '';
  showPage('page-dashboard');
  loadJobs();
}

// ─── Auth: Tabs ──────────────────────────────────────────────────────────────

document.querySelectorAll('.auth-tab').forEach(tab => {
  tab.addEventListener('click', () => {
    document.querySelectorAll('.auth-tab').forEach(t => t.classList.remove('active'));
    tab.classList.add('active');
    const target = tab.dataset.tab;
    document.querySelectorAll('.auth-form').forEach(f => f.classList.remove('active'));
    document.getElementById(`form-${target}`)?.classList.add('active');
    document.querySelectorAll('.form-error').forEach(e => e.classList.remove('visible'));
  });
});

// ─── Auth: Registro ──────────────────────────────────────────────────────────

document.getElementById('form-register').addEventListener('submit', async (e) => {
  e.preventDefault();
  const btn = document.getElementById('btn-register');
  const errEl = document.getElementById('register-error');

  const password = document.getElementById('reg-password').value;

  if (password.length < 8) {
    alert("¡Atención! La contraseña debe tener al menos 8 caracteres.");
    toast('error', 'Contraseña muy corta', 'Debe tener al menos 8 caracteres');
    return;
  }

  btn.disabled = true;
  btn.innerHTML = '<span class="spinner"></span> Registrando...';
  errEl.classList.remove('visible');

  try {
    const data = await apiRequest('POST', '/auth/register', {
      username: document.getElementById('reg-username').value.trim(),
      email:    document.getElementById('reg-email').value.trim(),
      password: password,
    });

    authToken = data.token;
    currentUser = data.user;
    localStorage.setItem('okf_token', authToken);
    localStorage.setItem('okf_user', JSON.stringify(currentUser));

    toast('success', '¡Registro exitoso!', `Bienvenido, ${currentUser.username}`);
    showDashboard();
  } catch (err) {
    errEl.textContent = err.message;
    errEl.classList.add('visible');
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

  const password = document.getElementById('login-password').value;

  btn.disabled = true;
  btn.innerHTML = '<span class="spinner"></span> Ingresando...';
  errEl.classList.remove('visible');

  try {
    const data = await apiRequest('POST', '/auth/login', {
      email:    document.getElementById('login-email').value.trim(),
      password: password,
    });

    authToken = data.token;
    currentUser = data.user;
    localStorage.setItem('okf_token', authToken);
    localStorage.setItem('okf_user', JSON.stringify(currentUser));

    toast('success', '¡Bienvenido!', currentUser.username);
    showDashboard();
  } catch (err) {
    errEl.textContent = err.message;
    errEl.classList.add('visible');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Iniciar sesión';
  }
});

// ─── Logout ──────────────────────────────────────────────────────────────────

document.getElementById('btn-logout').addEventListener('click', () => {
  authToken = null;
  currentUser = null;
  localStorage.removeItem('okf_token');
  localStorage.removeItem('okf_user');
  Object.values(pollingIntervals).forEach(clearInterval);
  pollingIntervals = {};
  toast('info', 'Sesión cerrada');
  showAuth();
});

// ─── Upload: Drag & Drop ─────────────────────────────────────────────────────

const uploadZone = document.getElementById('upload-zone');
const fileInput  = document.getElementById('file-input');

uploadZone.addEventListener('click', () => fileInput.click());

uploadZone.addEventListener('dragover', (e) => {
  e.preventDefault();
  uploadZone.classList.add('dragover');
});

uploadZone.addEventListener('dragleave', () => uploadZone.classList.remove('dragover'));

uploadZone.addEventListener('drop', (e) => {
  e.preventDefault();
  uploadZone.classList.remove('dragover');
  const file = e.dataTransfer.files[0];
  if (file) handleFileUpload(file);
});

fileInput.addEventListener('change', (e) => {
  const file = e.target.files[0];
  if (file) handleFileUpload(file);
  e.target.value = ''; // permitir seleccionar el mismo archivo de nuevo
});

// ─── Upload: Manejo principal ─────────────────────────────────────────────────

async function handleFileUpload(file) {
  const validExts = ['.md', '.markdown', '.html', '.htm', '.txt', '.pdf', '.docx'];
  const ext = '.' + file.name.split('.').pop().toLowerCase();

  if (!validExts.includes(ext)) {
    toast('error', 'Formato no soportado', `Usa: ${validExts.join(', ')}`);
    return;
  }

  if (file.size > 50 * 1024 * 1024) {
    toast('error', 'Archivo muy grande', 'El límite es 50 MB');
    return;
  }

  const progressEl = document.getElementById('upload-progress');
  const progressBar = document.getElementById('progress-bar');
  const progressLabel = document.getElementById('progress-label');
  const progressJobId = document.getElementById('progress-job-id');

  progressLabel.textContent = `Subiendo ${file.name}...`;
  progressJobId.textContent = '';
  progressBar.style.width = '30%';
  progressEl.classList.add('visible');

  try {
    const formData = new FormData();
    formData.append('file', file);

    progressBar.style.width = '60%';

    const data = await apiRequest('POST', '/documents', formData, true);

    progressBar.style.width = '100%';
    progressLabel.textContent = '✅ Documento recibido — procesando en segundo plano';
    progressJobId.textContent = `Job ID: ${data.job_id}`;

    toast('success', 'Documento recibido', 'El worker procesará tu documento. Puedes cerrar esta conexión.');

    // Iniciar polling del estado del job
    startPolling(data.job_id);

    // Recargar lista de jobs
    setTimeout(loadJobs, 1000);

    setTimeout(() => {
      progressEl.classList.remove('visible');
      progressBar.style.width = '0';
    }, 5000);

  } catch (err) {
    progressEl.classList.remove('visible');
    progressBar.style.width = '0';
    toast('error', 'Error al subir', err.message);
  }
}

// ─── Jobs: Cargar lista ──────────────────────────────────────────────────────

async function loadJobs() {
  try {
    const data = await apiRequest('GET', '/jobs');
    renderJobs(data.jobs || []);
    loadMetrics();
  } catch (err) {
    console.error('Error cargando jobs:', err);
  }
}

async function loadMetrics() {
  try {
    const metrics = await apiRequest('GET', '/metrics');
    let total = 0;
    let completed = 0;
    let pending = 0;
    let failed = 0;
    for (const [status, count] of Object.entries(metrics)) {
      total += count;
      if (status === 'completed') completed += count;
      if (status === 'pending' || status === 'processing') pending += count;
      if (status === 'failed' || status === 'canceled') failed += count;
    }
    document.getElementById('stat-total').textContent     = total;
    document.getElementById('stat-completed').textContent = completed;
    document.getElementById('stat-pending').textContent   = pending;
    document.getElementById('stat-failed').textContent    = failed;
  } catch (err) {
    console.error('Error cargando métricas:', err);
  }
}



function renderJobs(jobs) {
  const container = document.getElementById('jobs-list');

  if (jobs.length === 0) {
    container.innerHTML = `
      <div class="empty-state">
        <div class="empty-state-icon">📭</div>
        <h3>No hay trabajos aún</h3>
        <p>Sube tu primer documento para comenzar la conversión a bundle OKF</p>
      </div>
    `;
    return;
  }

  container.innerHTML = jobs.map(job => jobCardHTML(job)).join('');
}

function jobCardHTML(job) {
  const icons = {
    pending:    '⏳',
    processing: '⚙️',
    completed:  '✅',
    failed:     '❌',
    canceled:   '🚫',
  };

  const formatLabels = {
    markdown:  'Markdown',
    html:      'HTML',
    plaintext: 'Texto',
  };

  const dateStr = new Date(job.created_at).toLocaleString('es-CO', {
    day: '2-digit', month: '2-digit', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  });

  const processingDot = job.status === 'processing'
    ? '<span class="processing-dot"></span>' : '';

  const conceptsText = job.concept_count > 0
    ? `· ${job.concept_count} concepto${job.concept_count !== 1 ? 's' : ''}` : '';

  const sizeText = job.bundle_size > 0
    ? `· ${formatSize(job.bundle_size)}` : '';

  const cancelBtn = (job.status === 'pending' || job.status === 'processing')
    ? `<button class="btn btn-secondary btn-icon" onclick="cancelJob('${job.id}')" title="Cancelar trabajo">🚫</button>`
    : '';

  const retryBtn = (job.status === 'failed' || job.status === 'canceled')
    ? `<button class="btn btn-primary btn-icon" onclick="retryJob('${job.id}')" title="Reintentar trabajo">🔄</button>`
    : '';

  const downloadBtn = job.status === 'completed'
    ? `<button class="btn btn-success btn-icon" onclick="downloadBundle('${job.id}')" title="Descargar bundle ZIP">⬇️</button>`
    : '';

  const errorText = job.error_message
    ? `<span title="${job.error_message}" style="color:var(--accent-error)">⚠️</span>` : '';

  return `
    <div class="job-card" id="job-${job.id}">
      <div class="job-icon status-${job.status}">${icons[job.status] || '📄'}</div>
      <div class="job-info">
        <div class="job-filename">${escapeHtml(job.original_filename || job.filename)}</div>
        <div class="job-meta">
          <span>${formatLabels[job.format] || job.format}</span>
          <span>${dateStr}</span>
          <span>${conceptsText}</span>
          <span>${sizeText}</span>
          ${errorText}
        </div>
      </div>
      <span class="job-status-badge badge-${job.status}">
        ${processingDot}${job.status}
      </span>
      <div class="job-actions">
        ${cancelBtn}
        ${retryBtn}
        ${downloadBtn}
      </div>
    </div>
  `;
}

// ─── Polling de estado ────────────────────────────────────────────────────────

function startPolling(jobId) {
  if (pollingIntervals[jobId]) return;

  pollingIntervals[jobId] = setInterval(async () => {
    try {
      const job = await apiRequest('GET', `/jobs/${jobId}`);

      // Actualizar tarjeta individual si existe
      const cardEl = document.getElementById(`job-${jobId}`);
      if (cardEl) {
        cardEl.outerHTML = jobCardHTML(job);
      }

      // Actualizar stats
      loadJobs();

      if (job.status === 'completed') {
        clearInterval(pollingIntervals[jobId]);
        delete pollingIntervals[jobId];
        toast('success', '¡Bundle listo!',
          `${job.original_filename} convertido con ${job.concept_count} concepto(s)`);
      } else if (job.status === 'failed') {
        clearInterval(pollingIntervals[jobId]);
        delete pollingIntervals[jobId];
        toast('error', 'Conversión fallida', job.error_message || 'Error desconocido');
      }
    } catch (err) {
      console.error(`Polling error para job ${jobId}:`, err);
    }
  }, 3000); // Polling cada 3 segundos
}

// ─── Cancel y Retry ──────────────────────────────────────────────────────────

async function cancelJob(jobId) {
  try {
    await apiRequest('DELETE', `/jobs/${jobId}`);
    toast('info', 'Trabajo cancelado');
    loadJobs();
  } catch (err) {
    toast('error', 'Error al cancelar', err.message);
  }
}

async function retryJob(jobId) {
  try {
    const data = await apiRequest('POST', `/jobs/${jobId}/retry`);
    toast('success', 'Trabajo reintentado', data.message);
    startPolling(data.job_id);
    loadJobs();
  } catch (err) {
    toast('error', 'Error al reintentar', err.message);
  }
}

// ─── Download Bundle ──────────────────────────────────────────────────────────

async function downloadBundle(jobId) {
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

    toast('success', 'Descarga iniciada', 'El bundle ZIP se está descargando');
  } catch (err) {
    toast('error', 'Error al descargar', err.message);
  }
}

// ─── Refresh manual ──────────────────────────────────────────────────────────

document.getElementById('btn-refresh').addEventListener('click', () => {
  loadJobs();
  toast('info', 'Lista actualizada');
});

// ─── Utilidades ──────────────────────────────────────────────────────────────

function formatSize(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function escapeHtml(text) {
  const div = document.createElement('div');
  div.appendChild(document.createTextNode(text));
  return div.innerHTML;
}

// ─── Inicialización ───────────────────────────────────────────────────────────

function init() {
  if (authToken && currentUser) {
    showDashboard();
    // Reanudar polling de jobs en procesamiento
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
