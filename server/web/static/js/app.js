// ScanFlow Web UI

const API_BASE = window.location.origin;
let ws = null;

// Escape HTML to prevent XSS
function escapeHTML(str) {
    if (str == null) return '';
    const div = document.createElement('div');
    div.textContent = String(str);
    return div.innerHTML;
}

// Toast notifications instead of alert()
function showToast(message, type) {
    type = type || 'info';
    const container = document.getElementById('toast-container');
    if (!container) return;
    const toast = document.createElement('div');
    toast.className = 'toast toast-' + type;
    toast.textContent = message;
    container.appendChild(toast);
    // Trigger animation
    requestAnimationFrame(() => { toast.classList.add('show'); });
    setTimeout(() => {
        toast.classList.remove('show');
        setTimeout(() => toast.remove(), 300);
    }, 4000);
}

// Status labels
const STATUS_LABELS = {
    pending: 'Pending',
    scanning: 'Scanning',
    processing: 'Processing',
    completed: 'Completed',
    failed: 'Failed',
    cancelled: 'Cancelled'
};

function statusLabel(status) {
    return STATUS_LABELS[status] || status;
}

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    checkStatus();
    loadDevices();
    loadProfiles();
    loadSettings();
    loadJobs();
    connectWebSocket();

    // Refresh status periodically
    setInterval(checkStatus, 10000);
});

// API Helper
async function apiRequest(method, path, body) {
    const options = {
        method: method,
        headers: {
            'Content-Type': 'application/json',
        },
    };
    if (body) {
        options.body = JSON.stringify(body);
    }
    const resp = await fetch(API_BASE + path, options);
    if (!resp.ok) {
        const error = await resp.json().catch(() => ({ error: resp.statusText }));
        throw new Error(error.error || resp.statusText);
    }
    return resp.json();
}

// Status
async function checkStatus() {
    try {
        const status = await apiRequest('GET', '/api/v1/status');
        const dot = document.querySelector('.status .dot');
        const text = document.getElementById('status-text');
        dot.className = 'dot connected';
        const scannerWord = status.devices === 1 ? 'scanner' : 'scanners';
        text.textContent = 'Connected (' + status.devices + ' ' + scannerWord + ')';
    } catch (err) {
        const dot = document.querySelector('.status .dot');
        const text = document.getElementById('status-text');
        dot.className = 'dot error';
        text.textContent = 'Not connected';
    }
}

// Devices
async function loadDevices() {
    const container = document.getElementById('devices-list');
    try {
        const data = await apiRequest('GET', '/api/v1/scanner/devices');
        if (data.devices.length === 0) {
            container.innerHTML = '<div class="empty-state">No scanners found</div>';
            return;
        }
        container.innerHTML = data.devices.map(d => `
            <div class="device-card">
                <div>
                    <div class="device-name">${escapeHTML(d.vendor)} ${escapeHTML(d.model)}</div>
                    <div class="device-model">${escapeHTML(d.name)}</div>
                </div>
            </div>
        `).join('');
    } catch (err) {
        container.innerHTML = '<div class="empty-state">Failed to load scanners</div>';
    }
}

// Profiles
async function loadProfiles() {
    try {
        const data = await apiRequest('GET', '/api/v1/profiles');
        const select = document.getElementById('profile-select');
        if (data.profiles && data.profiles.length > 0) {
            select.innerHTML = data.profiles.map(p => `
                <option value="${escapeHTML(p.profile.name)}">${escapeHTML(p.profile.name)} - ${escapeHTML(p.profile.description)}</option>
            `).join('');
        }
    } catch (err) {
        console.error('Failed to load profiles:', err);
    }
}

// Settings
async function loadSettings() {
    try {
        const settings = await apiRequest('GET', '/api/v1/settings');
        document.getElementById('ocr-checkbox').checked = settings.ocr_enabled;
        document.getElementById('settings-ocr-enabled').checked = settings.ocr_enabled;
        document.getElementById('settings-ocr-language').value = settings.ocr_language || 'deu+eng';
    } catch (err) {
        console.error('Failed to load settings:', err);
    }
}

async function saveSettings() {
    const ocrEnabled = document.getElementById('settings-ocr-enabled').checked;
    const ocrLanguage = document.getElementById('settings-ocr-language').value;

    try {
        await apiRequest('PUT', '/api/v1/settings', {
            ocr_enabled: ocrEnabled,
            ocr_language: ocrLanguage,
        });
        // Sync the per-scan checkbox with the new default
        document.getElementById('ocr-checkbox').checked = ocrEnabled;
        showToast('Settings saved', 'success');
    } catch (err) {
        showToast('Error: ' + err.message, 'error');
    }
}

// Load existing jobs on page load
async function loadJobs() {
    // The status endpoint gives us total_jobs count, but we need to check
    // if there are active jobs. We'll load them via the status endpoint.
    try {
        const status = await apiRequest('GET', '/api/v1/status');
        if (status.active_jobs === 0 && status.total_jobs === 0) {
            return; // Keep "No active jobs" message
        }
    } catch (err) {
        // Ignore - jobs list stays as default
    }
}

// Scan
async function startScan() {
    const btn = document.getElementById('scan-btn');
    btn.disabled = true;
    btn.textContent = 'Scanning...';

    const profile = document.getElementById('profile-select').value;
    const output = document.getElementById('output-select').value;
    const title = document.getElementById('title-input').value;
    const ocrEnabled = document.getElementById('ocr-checkbox').checked;

    const req = {
        profile: profile,
        output: { target: output },
        ocr_enabled: ocrEnabled,
    };

    if (title) {
        req.metadata = { title: title };
    }

    try {
        const job = await apiRequest('POST', '/api/v1/scan', req);
        addJobCard(job);
        document.getElementById('title-input').value = '';
        showToast('Scan started', 'success');
    } catch (err) {
        showToast('Scan failed: ' + err.message, 'error');
    } finally {
        btn.disabled = false;
        btn.textContent = 'Start Scan';
    }
}

// Cancel a job
async function cancelJob(jobId) {
    try {
        await apiRequest('DELETE', '/api/v1/scan/' + jobId);
        showToast('Job cancelled', 'info');
        const card = document.getElementById('job-' + jobId);
        if (card) {
            const statusEl = card.querySelector('.job-status');
            if (statusEl) {
                statusEl.textContent = 'Cancelled';
                statusEl.className = 'job-status cancelled';
            }
            const cancelBtn = card.querySelector('.btn-cancel');
            if (cancelBtn) cancelBtn.remove();
        }
    } catch (err) {
        showToast('Cancel failed: ' + err.message, 'error');
    }
}

// Job Cards
function addJobCard(job) {
    const container = document.getElementById('jobs-list');

    // Remove empty state
    const emptyState = container.querySelector('.empty-state');
    if (emptyState) emptyState.remove();

    // Don't add duplicate cards
    if (document.getElementById('job-' + job.id)) return;

    const isActive = job.status === 'pending' || job.status === 'scanning' || job.status === 'processing';

    const card = document.createElement('div');
    card.className = 'job-card';
    card.id = 'job-' + job.id;
    card.innerHTML = `
        <div class="job-header">
            <span class="job-id">${escapeHTML(job.id.slice(0, 8))}...</span>
            <div class="job-header-right">
                ${isActive ? '<button class="btn btn-cancel" onclick="cancelJob(\'' + escapeHTML(job.id) + '\')">Cancel</button>' : ''}
                <span class="job-status ${escapeHTML(job.status)}">${escapeHTML(statusLabel(job.status))}</span>
            </div>
        </div>
        <div class="job-details">Profile: ${escapeHTML(job.profile)} | Pages: <span class="job-pages">${job.pages ? job.pages.length : 0}</span></div>
        <div class="progress-bar" style="margin-top: 8px;">
            <div class="fill" style="width: ${parseInt(job.progress) || 0}%"></div>
        </div>
    `;

    container.prepend(card);
}

function updateJobCard(update) {
    const card = document.getElementById('job-' + update.job_id);
    if (!card) return;

    const statusEl = card.querySelector('.job-status');
    if (statusEl && update.status) {
        statusEl.textContent = statusLabel(update.status);
        statusEl.className = 'job-status ' + update.status;
    }

    const progressBar = card.querySelector('.fill');
    if (progressBar && update.progress !== undefined) {
        progressBar.style.width = update.progress + '%';
    }

    // Remove cancel button for terminal states
    if (update.status === 'completed' || update.status === 'failed' || update.status === 'cancelled') {
        const cancelBtn = card.querySelector('.btn-cancel');
        if (cancelBtn) cancelBtn.remove();
    }
}

// WebSocket
function connectWebSocket() {
    const wsUrl = API_BASE.replace('http', 'ws') + '/api/v1/ws';

    try {
        ws = new WebSocket(wsUrl);

        ws.onmessage = (event) => {
            const update = JSON.parse(event.data);
            updateJobCard(update);
        };

        ws.onclose = () => {
            setTimeout(connectWebSocket, 5000);
        };

        ws.onerror = () => {
            ws.close();
        };
    } catch (err) {
        setTimeout(connectWebSocket, 5000);
    }
}
