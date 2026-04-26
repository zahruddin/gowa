const API = '';
let activeSessions = []; // Menyimpan daftar session untuk dropdown

// --- TABS ---
function switchTab(tab) {
  document.querySelectorAll('[id^=tab-]').forEach(el => el.classList.add('hidden'));
  document.getElementById('tab-'+tab).classList.remove('hidden');
  document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.classList.remove('active');
    btn.classList.add('text-slate-400');
  });
  document.querySelector(`[data-tab="${tab}"]`).classList.add('active');
  document.querySelector(`[data-tab="${tab}"]`).classList.remove('text-slate-400');
  
  if(tab==='sessions') loadSessions();
  if(tab==='katalog') loadKatalog();
  if(tab==='whitelist') loadWhitelist();
  if(tab==='bulk') loadBulkJobs();
}

// --- MODAL ---
function openModal(title, bodyHTML) {
  document.getElementById('modal-title').textContent = title;
  document.getElementById('modal-body').innerHTML = bodyHTML;
  const m = document.getElementById('modal');
  m.classList.remove('hidden');
  m.classList.add('flex');
}
function closeModal() {
  const m = document.getElementById('modal');
  m.classList.add('hidden');
  m.classList.remove('flex');
}
document.getElementById('modal').addEventListener('click', e => { if(e.target.id==='modal') closeModal(); });

// --- UPDATE DROPDOWNS ---
function updateDropdowns() {
  const options = activeSessions.length > 0 
    ? activeSessions.map(s => `<option value="${s.id}">${s.id} ${s.is_connected ? '🟢' : '🔴'}</option>`).join('')
    : `<option value="">No Sessions Found</option>`;
  
  const katSelect = document.getElementById('katalog-session-select');
  const wlSelect = document.getElementById('whitelist-session-select');
  const bulkSelect = document.getElementById('bulk-session-select');
  
  const currentKat = katSelect ? katSelect.value : null;
  const currentWl = wlSelect ? wlSelect.value : null;
  const currentBulk = bulkSelect ? bulkSelect.value : null;

  if (katSelect) katSelect.innerHTML = options;
  if (wlSelect) wlSelect.innerHTML = options;
  if (bulkSelect) bulkSelect.innerHTML = options;

  if(currentKat && katSelect) katSelect.value = currentKat;
  if(currentWl && wlSelect) wlSelect.value = currentWl;
  if(currentBulk && bulkSelect) bulkSelect.value = currentBulk;
}

// --- SESSIONS ---
let qrPollers = {};

async function loadSessions() {
  const el = document.getElementById('sessions-list');
  try {
    const res = await fetch(API+'/api/sessions');
    const data = await res.json();
    
    // Update global session state for dropdowns
    activeSessions = data || [];
    updateDropdowns();

    if(!data || data.length===0) {
      el.innerHTML = '<p class="text-slate-500 col-span-full text-center py-12">No sessions yet. Click "+ Add Session" to start.</p>';
      return;
    }

    el.innerHTML = data.map(s => {
      const connected = s.is_connected;
      const statusColor = connected ? 'emerald' : 'red';
      const statusText = connected ? 'Connected' : (s.status||'Offline');
      const name = s.push_name || s.device_jid || s.id;
      const token = s.api_token ? s.api_token.substring(0,16)+'...' : '-';
      return `
      <div class="bg-slate-900 border border-slate-800 rounded-xl p-5 hover:border-slate-600 transition">
        <div class="flex justify-between items-start mb-3">
          <div class="w-10 h-10 bg-${statusColor}-500/20 rounded-lg flex items-center justify-center text-${statusColor}-400 text-lg">📱</div>
          <span class="bg-${statusColor}-500/10 text-${statusColor}-400 text-xs px-2 py-1 rounded-full border border-${statusColor}-500/20">${statusText}</span>
        </div>
        <h3 class="font-bold text-base mb-0.5">${name}</h3>
        <p class="text-slate-500 text-xs mb-3">ID: ${s.id}</p>
        <div class="space-y-1.5 text-xs mb-4">
          <div class="flex justify-between"><span class="text-slate-500">Token</span><code class="bg-slate-800 px-1.5 py-0.5 rounded text-blue-400">${token}</code></div>
          <div class="flex justify-between"><span class="text-slate-500">Webhook</span><span class="text-slate-300">${s.webhook_url||'Not set'}</span></div>
        </div>
        <div class="flex gap-2">
          ${connected
            ? `<button onclick="disconnectSession('${s.id}')" class="flex-1 bg-amber-500/10 hover:bg-amber-500/20 text-amber-400 py-2 rounded-lg text-xs font-semibold transition">Disconnect</button>`
            : `<button onclick="connectSession('${s.id}')" class="flex-1 bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-400 py-2 rounded-lg text-xs font-semibold transition">Connect</button>`
          }
          <button onclick="showSessionDetail('${s.id}','${s.api_token||''}','${s.webhook_url||''}')" class="flex-1 bg-slate-800 hover:bg-slate-700 py-2 rounded-lg text-xs font-semibold transition">Detail</button>
          <button onclick="deleteSession('${s.id}')" class="bg-red-500/10 hover:bg-red-500/20 text-red-400 px-3 py-2 rounded-lg transition text-xs">🗑</button>
        </div>
      </div>`;
    }).join('');
  } catch(e) {
    el.innerHTML = '<p class="text-red-400 col-span-full text-center py-12">Error: '+e.message+'</p>';
  }
}

// ... [Fungsi Create Session, QR Code, dan Delete Session sama persis seperti sebelumnya] ...
function showAddSession() {
  openModal('Add New Session', `
    <div class="space-y-3">
      <input id="new-session-id" placeholder="Session Name (e.g. store1)" class="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-blue-500">
      <div id="create-btns" class="flex gap-2">
        <button onclick="createSession()" class="flex-1 bg-blue-600 hover:bg-blue-700 py-2 rounded-lg text-sm font-semibold transition">Create</button>
        <button onclick="closeModal()" class="flex-1 bg-slate-700 hover:bg-slate-600 py-2 rounded-lg text-sm font-semibold transition">Cancel</button>
      </div>
      <div id="qr-area" class="hidden mt-4 text-center">
        <p class="text-sm text-emerald-400 mb-3 font-medium">📱 Scan QR with WhatsApp</p>
        <div id="qr-target" class="inline-block bg-white p-3 rounded-xl"></div>
        <p id="qr-status" class="text-xs text-slate-500 mt-3">Waiting for QR...</p>
        <button onclick="closeModal();loadSessions();" class="mt-4 bg-slate-700 hover:bg-slate-600 px-4 py-2 rounded-lg text-sm">Close</button>
      </div>
    </div>
  `);
}

async function createSession() {
  const id = document.getElementById('new-session-id').value.trim();
  if(!id) return alert('Session name is required');
  const res = await fetch(API+'/api/sessions/create', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({id})});
  const data = await res.json();
  if(data.error) return alert(data.error);

  document.getElementById('create-btns').classList.add('hidden');
  document.getElementById('new-session-id').classList.add('hidden');
  document.getElementById('qr-area').classList.remove('hidden');
  pollQR(id);
}

function showQRModal(sessionID) {
  openModal('QR Code: '+sessionID, `
    <div class="text-center">
      <p class="text-sm text-emerald-400 mb-3 font-medium">📱 Scan QR with WhatsApp</p>
      <div id="qr-target" class="inline-block bg-white p-3 rounded-xl"></div>
      <p id="qr-status" class="text-xs text-slate-500 mt-3">Waiting for QR...</p>
      <button onclick="closeModal();loadSessions();" class="mt-4 bg-slate-700 hover:bg-slate-600 px-4 py-2 rounded-lg text-sm">Close</button>
    </div>
  `);
  pollQR(sessionID);
}

function pollQR(sessionID) {
  if(qrPollers[sessionID]) clearInterval(qrPollers[sessionID]);
  let lastQR = '';
  qrPollers[sessionID] = setInterval(async () => {
    try {
      const res = await fetch(API+'/api/sessions/qr?id='+encodeURIComponent(sessionID));
      const data = await res.json();
      const target = document.getElementById('qr-target');
      const status = document.getElementById('qr-status');
      if(data.has_qr && target && data.qr !== lastQR) {
        lastQR = data.qr;
        target.innerHTML = '';
        try {
          new QRCode(target, { text: data.qr, width: 240, height: 240, colorDark: '#000000', colorLight: '#ffffff', correctLevel: QRCode.CorrectLevel.L });
        } catch(qrErr) {
          const img = document.createElement('img');
          img.src = 'https://api.qrserver.com/v1/create-qr-code/?size=240x240&data='+encodeURIComponent(data.qr);
          target.appendChild(img);
        }
        if(status) status.textContent = 'QR ready — scan now!';
      } else if(!data.has_qr && lastQR !== '') {
        clearInterval(qrPollers[sessionID]);
        delete qrPollers[sessionID];
        closeModal();
        loadSessions();
      }
    } catch(e) {}
  }, 1500);
}

async function deleteSession(id) {
  if(!confirm('Delete session "'+id+'"? This will logout from WhatsApp.')) return;
  await fetch(API+'/api/sessions/delete', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({id})});
  loadSessions();
}
async function connectSession(id) {
  await fetch(API+'/api/sessions/connect', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({id})});
  showQRModal(id);
  setTimeout(() => { if(!qrPollers[id]) { closeModal(); loadSessions(); } }, 3000);
}
async function disconnectSession(id) {
  await fetch(API+'/api/sessions/disconnect', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({id})});
  loadSessions();
}

function showSessionDetail(id, token, webhook) {
  openModal('Session: '+id, `
    <div class="space-y-3 text-sm">
      <div><span class="text-slate-500">API Token:</span>
        <code class="block bg-slate-900 p-2 rounded mt-1 text-blue-400 text-xs break-all select-all">${token}</code>
      </div>
      <div>
        <label class="text-slate-500 block mb-1">Webhook URL:</label>
        <input id="edit-webhook" value="${webhook}" placeholder="https://example.com/webhook" class="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-blue-500">
      </div>
      <div class="flex gap-2">
        <button onclick="saveWebhook('${id}')" class="flex-1 bg-blue-600 hover:bg-blue-700 py-2 rounded-lg font-semibold transition">Save Webhook</button>
        <button onclick="closeModal()" class="flex-1 bg-slate-700 hover:bg-slate-600 py-2 rounded-lg font-semibold transition">Close</button>
      </div>
    </div>
  `);
}

async function saveWebhook(id) {
  const url = document.getElementById('edit-webhook').value;
  await fetch(API+'/api/sessions/webhook', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({id, webhook_url:url})});
  closeModal();
  loadSessions();
}

// --- KATALOG ---
async function loadKatalog() {
  const sessionID = document.getElementById('katalog-session-select').value;
  const btnAdd = document.getElementById('btn-add-katalog');
  const el = document.getElementById('katalog-list');

  if(!sessionID) {
    btnAdd.disabled = true;
    btnAdd.classList.add('opacity-50', 'cursor-not-allowed');
    el.innerHTML = '<p class="text-slate-500 text-center py-8">Please select a session first to view or add katalog.</p>';
    
    document.getElementById('btn-export-katalog').classList.add('opacity-50', 'cursor-not-allowed', 'pointer-events-none');
    document.getElementById('import-katalog-file').parentElement.classList.add('opacity-50', 'cursor-not-allowed', 'pointer-events-none');
    document.getElementById('import-katalog-file').disabled = true;
    
    return;
  }

  btnAdd.disabled = false;
  btnAdd.classList.remove('opacity-50', 'cursor-not-allowed');

  const btnExport = document.getElementById('btn-export-katalog');
  btnExport.classList.remove('opacity-50', 'cursor-not-allowed', 'pointer-events-none');
  btnExport.href = API + '/api/katalog/export?session_id=' + encodeURIComponent(sessionID);

  const importFile = document.getElementById('import-katalog-file');
  importFile.parentElement.classList.remove('opacity-50', 'cursor-not-allowed', 'pointer-events-none');
  importFile.disabled = false;

  // PERBAIKAN: Kirim parameter session_id ke backend
  const res = await fetch(API+'/api/katalog?session_id='+encodeURIComponent(sessionID));
  const data = await res.json();
  
  if(!data||data.length===0) {
    el.innerHTML = `<p class="text-slate-500 text-center py-8">No katalog entries for <b>${sessionID}</b>.</p>`;
    return;
  }
  
  el.innerHTML = data.map(k => `
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-4 flex justify-between items-start gap-4">
      <div class="flex-1 min-w-0">
        <span class="font-semibold text-emerald-400 text-sm">${k.keyword}</span>
        <p class="text-slate-400 text-xs mt-1 whitespace-pre-wrap break-words">${k.details}</p>
      </div>
      <div class="flex gap-2 shrink-0">
        <button onclick="showEditKatalog('${k.keyword}',\`${k.details.replace(/`/g,"\\`").replace(/\\/g,"\\\\")}\`)" class="bg-slate-800 hover:bg-slate-700 px-3 py-1.5 rounded text-xs transition">Edit</button>
        <button onclick="deleteKatalog('${k.keyword}')" class="bg-red-500/10 hover:bg-red-500/20 text-red-400 px-3 py-1.5 rounded text-xs transition">Delete</button>
      </div>
    </div>
  `).join('');
}

function showAddKatalog() {
  const sessionID = document.getElementById('katalog-session-select').value;
  openModal('Add Katalog ('+sessionID+')', `
    <div class="space-y-3">
      <input id="kat-keyword" placeholder="Keyword" class="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-blue-500">
      <textarea id="kat-details" rows="4" placeholder="Response message..." class="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-blue-500"></textarea>
      <div class="flex gap-2">
        <button onclick="saveKatalog()" class="flex-1 bg-emerald-600 hover:bg-emerald-700 py-2 rounded-lg text-sm font-semibold transition">Save</button>
        <button onclick="closeModal()" class="flex-1 bg-slate-700 hover:bg-slate-600 py-2 rounded-lg text-sm font-semibold transition">Cancel</button>
      </div>
    </div>
  `);
}

function showEditKatalog(keyword, details) {
  const sessionID = document.getElementById('katalog-session-select').value;
  openModal('Edit Katalog ('+sessionID+')', `
    <div class="space-y-3">
      <input id="kat-keyword" value="${keyword}" readonly class="w-full bg-slate-950 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-500">
      <textarea id="kat-details" rows="4" class="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-blue-500">${details}</textarea>
      <div class="flex gap-2">
        <button onclick="saveKatalog()" class="flex-1 bg-emerald-600 hover:bg-emerald-700 py-2 rounded-lg text-sm font-semibold transition">Update</button>
        <button onclick="closeModal()" class="flex-1 bg-slate-700 hover:bg-slate-600 py-2 rounded-lg text-sm font-semibold transition">Cancel</button>
      </div>
    </div>
  `);
}

async function saveKatalog() {
  const sessionID = document.getElementById('katalog-session-select').value;
  const keyword = document.getElementById('kat-keyword').value;
  const details = document.getElementById('kat-details').value;
  
  // PERBAIKAN: Sertakan session_id di body POST
  await fetch(API+'/api/katalog/save', {
    method:'POST', 
    headers:{'Content-Type':'application/json'}, 
    body:JSON.stringify({session_id: sessionID, keyword, details})
  });
  closeModal();
  loadKatalog();
}

async function deleteKatalog(keyword) {
  const sessionID = document.getElementById('katalog-session-select').value;
  if(!confirm(`Delete keyword "${keyword}" from session ${sessionID}?`)) return;
  
  await fetch(API+'/api/katalog/delete', {
    method:'POST', 
    headers:{'Content-Type':'application/json'}, 
    body:JSON.stringify({session_id: sessionID, keyword})
  });
  loadKatalog();
}

async function handleImportCSV(event) {
  const sessionID = document.getElementById('katalog-session-select').value;
  if (!sessionID) {
    alert("Please select a session first");
    return;
  }
  const file = event.target.files[0];
  if (!file) return;

  const formData = new FormData();
  formData.append('session_id', sessionID);
  formData.append('file', file);

  try {
    const res = await fetch(API+'/api/katalog/import', {
      method: 'POST',
      body: formData
    });
    const data = await res.json();
    if (data.status === 'success') {
      alert(data.message || 'Import successful');
      loadKatalog();
    } else {
      alert('Error: ' + (data.error || 'Unknown error'));
    }
  } catch(e) {
    alert('Failed to import: ' + e.message);
  } finally {
    event.target.value = ''; // Reset input
  }
}

// --- WHITELIST ---
async function loadWhitelist() {
  const sessionID = document.getElementById('whitelist-session-select').value;
  const btnAdd = document.getElementById('btn-add-whitelist');
  const el = document.getElementById('whitelist-list');

  if(!sessionID) {
    btnAdd.disabled = true;
    btnAdd.classList.add('opacity-50', 'cursor-not-allowed');
    el.innerHTML = '<p class="text-slate-500 text-center py-8">Please select a session first to view or add whitelist.</p>';
    return;
  }

  btnAdd.disabled = false;
  btnAdd.classList.remove('opacity-50', 'cursor-not-allowed');

  // PERBAIKAN: Kirim parameter session_id ke backend
  const res = await fetch(API+'/api/whitelist?session_id='+encodeURIComponent(sessionID));
  const data = await res.json();
  
  if(!data||data.length===0) {
    el.innerHTML = `<p class="text-slate-500 text-center py-8">No admin numbers for <b>${sessionID}</b>.</p>`;
    return;
  }
  
  el.innerHTML = data.map(w => `
    <div class="bg-slate-900 border border-slate-800 rounded-lg p-4 flex justify-between items-center">
      <div>
        <span class="font-semibold text-purple-400 text-sm">${w.number}</span>
        <span class="text-slate-500 text-xs ml-2">${w.name||''}</span>
      </div>
      <button onclick="deleteWhitelist('${w.number}')" class="bg-red-500/10 hover:bg-red-500/20 text-red-400 px-3 py-1.5 rounded text-xs transition">Delete</button>
    </div>
  `).join('');
}

function showAddWhitelist() {
  const sessionID = document.getElementById('whitelist-session-select').value;
  openModal('Add Admin ('+sessionID+')', `
    <div class="space-y-3">
      <input id="wl-number" placeholder="Number (e.g. 6281234567890)" class="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-blue-500">
      <input id="wl-name" placeholder="Name (optional)" class="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-blue-500">
      <div class="flex gap-2">
        <button onclick="saveWhitelist()" class="flex-1 bg-purple-600 hover:bg-purple-700 py-2 rounded-lg text-sm font-semibold transition">Save</button>
        <button onclick="closeModal()" class="flex-1 bg-slate-700 hover:bg-slate-600 py-2 rounded-lg text-sm font-semibold transition">Cancel</button>
      </div>
    </div>
  `);
}

async function saveWhitelist() {
  const sessionID = document.getElementById('whitelist-session-select').value;
  const number = document.getElementById('wl-number').value;
  const name = document.getElementById('wl-name').value;
  
  // PERBAIKAN: Sertakan session_id di body POST
  await fetch(API+'/api/whitelist/save', {
    method:'POST', 
    headers:{'Content-Type':'application/json'}, 
    body:JSON.stringify({session_id: sessionID, number, name})
  });
  closeModal();
  loadWhitelist();
}

async function deleteWhitelist(number) {
  const sessionID = document.getElementById('whitelist-session-select').value;
  if(!confirm(`Remove "${number}" from ${sessionID} whitelist?`)) return;
  
  await fetch(API+'/api/whitelist/delete', {
    method:'POST', 
    headers:{'Content-Type':'application/json'}, 
    body:JSON.stringify({session_id: sessionID, number})
  });
  loadWhitelist();
}

// --- BULK SENDER ---
async function loadBulkJobs() {
  const bulkSelect = document.getElementById('bulk-session-select');
  if (!bulkSelect) return;
  const sessionID = bulkSelect.value;
  let url = API+'/api/bulk/jobs';
  if(sessionID) url += '?session_id='+encodeURIComponent(sessionID);
  
  const res = await fetch(url);
  const data = await res.json();
  const el = document.getElementById('bulk-jobs-list');
  
  if(!data || data.length === 0) {
    el.innerHTML = '<tr><td colspan="5" class="py-12 text-center text-slate-500 italic">No broadcast history yet.</td></tr>';
    return;
  }

  el.innerHTML = data.map(j => {
    const statusClass = j.status === 'completed' ? 'text-emerald-400' : (j.status === 'failed' ? 'text-red-400' : 'text-blue-400');
    return `
      <tr class="hover:bg-slate-800/50 transition">
        <td class="py-4 text-slate-400 font-mono text-xs">#${j.id}</td>
        <td class="py-4 font-medium">${j.session_id}</td>
        <td class="py-4">
          <div class="flex items-center gap-2">
            <span class="text-xs text-slate-400">${j.success}/${j.total}</span>
            <div class="flex-1 h-1.5 bg-slate-800 rounded-full w-24 overflow-hidden">
              <div class="h-full bg-blue-500" style="width: ${(j.success/j.total*100)||0}%"></div>
            </div>
          </div>
        </td>
        <td class="py-4"><span class="text-xs px-2 py-0.5 rounded-full bg-slate-800 border border-slate-700 ${statusClass}">${j.status}</span></td>
        <td class="py-4 text-slate-500 text-xs">${new Date(j.created_at).toLocaleString()}</td>
      </tr>
    `;
  }).join('');
}

async function uploadBulk() {
  const bulkSelect = document.getElementById('bulk-session-select');
  if (!bulkSelect) return;
  const sessionID = bulkSelect.value;
  const fileInput = document.getElementById('bulk-file');
  const messageInput = document.getElementById('bulk-message');
  if (!fileInput || !messageInput) return;
  
  const message = messageInput.value;

  if(!sessionID || !fileInput.files[0] || !message) {
    return alert('Please fill all fields: Session, Excel File, and Message.');
  }

  const formData = new FormData();
  formData.append('session_id', sessionID);
  formData.append('file', fileInput.files[0]);
  formData.append('message', message);

  try {
    const res = await fetch(API+'/api/bulk/upload', {
      method: 'POST',
      body: formData
    });
    const data = await res.json();
    if(data.status === 'success') {
      alert('Broadcast job created!');
      loadBulkJobs();
    } else {
      alert('Error: ' + data.error);
    }
  } catch(e) {
    alert('Failed to upload: ' + e.message);
  }
}

// --- INIT ---
loadSessions(); // Memuat sesi pertama kali