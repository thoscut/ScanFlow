// ScanFlow Web UI

const API_BASE = window.location.origin;
let ws = null;

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    checkStatus();
    loadDevices();
    loadProfiles();
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
        text.textContent = `Verbunden (${status.devices} Scanner)`;
    } catch (err) {
        const dot = document.querySelector('.status .dot');
        const text = document.getElementById('status-text');
        dot.className = 'dot error';
        text.textContent = 'Nicht verbunden';
    }
}

// Devices
async function loadDevices() {
    const container = document.getElementById('devices-list');
    try {
        const data = await apiRequest('GET', '/api/v1/scanner/devices');
        if (data.devices.length === 0) {
            container.innerHTML = '<div class="empty-state">Kein Scanner gefunden</div>';
            return;
        }
        container.innerHTML = data.devices.map(d => `
            <div class="device-card">
                <div>
                    <div class="device-name">${d.vendor} ${d.model}</div>
                    <div class="device-model">${d.name}</div>
                </div>
            </div>
        `).join('');
    } catch (err) {
        container.innerHTML = '<div class="empty-state">Fehler beim Laden</div>';
    }
}

// Profiles
async function loadProfiles() {
    try {
        const data = await apiRequest('GET', '/api/v1/profiles');
        const select = document.getElementById('profile-select');
        select.innerHTML = data.profiles.map(p => `
            <option value="${p.profile.name}">${p.profile.name} - ${p.profile.description}</option>
        `).join('');
    } catch (err) {
        console.error('Failed to load profiles:', err);
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

    const req = {
        profile: profile,
        output: { target: output },
    };

    if (title) {
        req.metadata = { title: title };
    }

    try {
        const job = await apiRequest('POST', '/api/v1/scan', req);
        addJobCard(job);
        document.getElementById('title-input').value = '';
    } catch (err) {
        alert('Scan fehlgeschlagen: ' + err.message);
    } finally {
        btn.disabled = false;
        btn.textContent = 'Scan starten';
    }
}

// Job Cards
function addJobCard(job) {
    const container = document.getElementById('jobs-list');

    // Remove empty state
    const emptyState = container.querySelector('.empty-state');
    if (emptyState) emptyState.remove();

    const card = document.createElement('div');
    card.className = 'job-card';
    card.id = 'job-' + job.id;
    card.innerHTML = `
        <div class="job-header">
            <span class="job-id">${job.id.substring(0, 8)}</span>
            <span class="job-status ${job.status}">${job.status}</span>
        </div>
        <div>Profil: ${job.profile} | Seiten: ${job.pages ? job.pages.length : 0}</div>
        <div class="progress-bar" style="margin-top: 8px;">
            <div class="fill" style="width: ${job.progress || 0}%"></div>
        </div>
    `;

    container.prepend(card);
}

function updateJobCard(update) {
    const card = document.getElementById('job-' + update.job_id);
    if (!card) return;

    const statusEl = card.querySelector('.job-status');
    if (statusEl && update.status) {
        statusEl.textContent = update.status;
        statusEl.className = 'job-status ' + update.status;
    }

    const progressBar = card.querySelector('.fill');
    if (progressBar && update.progress !== undefined) {
        progressBar.style.width = update.progress + '%';
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
