const API_BASE = window.location.port === '5173' ? 'http://localhost:8080' : '';

const state = {
  prompt: 'Create a commercial ad for a night riding safety light in the city',
  assetsText: 'uploads/light.png,image\nuploads/night-ride.mp4,video',
  taskId: '',
  status: 'idle',
  script: null,
  error: '',
  rewriteText: {},
};

const root = document.getElementById('root');
render();

function render() {
  root.innerHTML = `
    <main class="studio-shell">
      <section class="control-pane">
        <div class="brand-row">
          <span class="icon-badge">▣</span>
          <div>
            <h1>CreatorScript Studio</h1>
            <p>AI short-video script and storyboard planning desk</p>
          </div>
        </div>

        <label class="field-label" for="prompt">Creative brief</label>
        <textarea id="prompt" class="prompt-box">${escapeHtml(state.prompt)}</textarea>

        <label class="field-label" for="assets">Assets, one per line: object_key,kind</label>
        <textarea id="assets" class="asset-box">${escapeHtml(state.assetsText)}</textarea>

        <div class="actions-row">
          <button id="generate" class="primary" ${isBusy() ? 'disabled' : ''}>Generate script</button>
          <button id="refresh" class="secondary" ${state.taskId ? '' : 'disabled'}>Refresh</button>
        </div>

        <div class="status-strip"><span>Status</span><strong>${escapeHtml(state.status)}</strong></div>
        ${state.taskId ? `<div class="task-id">Task ${escapeHtml(state.taskId)}</div>` : ''}
        ${state.error ? `<div class="error-box">${escapeHtml(state.error)}</div>` : ''}
      </section>

      <section class="document-pane">
        ${state.script ? renderScript(state.script) : renderEmpty()}
      </section>
    </main>`;

  document.getElementById('prompt').addEventListener('input', (event) => state.prompt = event.target.value);
  document.getElementById('assets').addEventListener('input', (event) => state.assetsText = event.target.value);
  document.getElementById('generate').addEventListener('click', generate);
  document.getElementById('refresh').addEventListener('click', () => state.taskId && loadScript(state.taskId));
  document.querySelectorAll('[data-export]').forEach((button) => button.addEventListener('click', exportMarkdown));
  document.querySelectorAll('[data-rewrite-input]').forEach((input) => {
    input.addEventListener('input', (event) => state.rewriteText[event.target.dataset.rewriteInput] = event.target.value);
  });
  document.querySelectorAll('[data-rewrite]').forEach((button) => {
    button.addEventListener('click', () => rewrite(button.dataset.rewrite, Number(button.dataset.index)));
  });
}

function renderEmpty() {
  return `
    <div class="empty-state">
      <div class="empty-icon">✦</div>
      <h2>Generate a production-ready script document</h2>
      <p>The result will be a readable script, storyboard table, quality review, and Markdown export. No raw planning JSON is shown here.</p>
    </div>`;
}

function renderScript(script) {
  return `
    <article class="script-doc">
      <div class="doc-header">
        <div>
          <div class="eyebrow">${escapeHtml(script.prompt_type)} · ${Number(script.duration_sec || 0)}s</div>
          <h2>${escapeHtml(script.title)}</h2>
          <p>${escapeHtml(script.logline)}</p>
        </div>
        <button class="secondary" data-export>Export Markdown</button>
      </div>

      <section>
        <h3>Synopsis</h3>
        <p>${escapeHtml(script.synopsis)}</p>
      </section>

      <div class="two-col">
        <section>
          <h3>Characters</h3>
          ${(script.characters || []).map((c) => `<p><b>${escapeHtml(c.name)}</b>: ${escapeHtml(c.description)}</p>`).join('')}
        </section>
        <section>
          <h3>Scenes</h3>
          ${(script.scenes || []).map((s) => `<p><b>${Number(s.index)}. ${escapeHtml(s.name)}</b> · ${escapeHtml(s.mood)}</p>`).join('')}
        </section>
      </div>

      <section>
        <h3>Storyboard</h3>
        <div class="storyboard-table">
          ${(script.storyboard_rows || []).map(renderRow).join('')}
        </div>
      </section>

      <section class="quality-card">
        <h3>Quality Review</h3>
        <div class="score">${Number(script.quality_review?.score || 0)}</div>
        <p>${script.quality_review?.passed ? 'Ready for review.' : 'Needs a human pass before production.'}</p>
        <p><b>Issues:</b> ${escapeHtml((script.quality_review?.issues || []).join('; ') || 'none')}</p>
        <p><b>Suggested fixes:</b> ${escapeHtml((script.quality_review?.suggested_fixes || []).join('; ') || 'none')}</p>
      </section>
    </article>`;
}

function renderRow(row) {
  const shotKey = `rewrite-shot-${row.index}`;
  const dialogueKey = `rewrite-dialogue-${row.index}`;
  return `
    <div class="story-row">
      <div class="time-chip">${escapeHtml(row.time_range)}</div>
      <div>
        <h4>Shot ${Number(row.index)} · ${escapeHtml(row.purpose)}</h4>
        <p>${escapeHtml(row.visual)}</p>
        <blockquote>${escapeHtml(row.voiceover || 'No voice-over yet.')}</blockquote>
        <small>${escapeHtml(row.asset_hint)}</small>
        <div class="rewrite-row">
          <input placeholder="Rewrite instruction" data-rewrite-input="${shotKey}" value="${escapeHtml(state.rewriteText[shotKey] || '')}" />
          <button data-rewrite="rewrite-shot" data-index="${Number(row.index)}">Rewrite shot</button>
          <button data-rewrite="rewrite-dialogue" data-index="${Number(row.index)}">Rewrite dialogue</button>
        </div>
      </div>
    </div>`;
}

async function generate() {
  state.error = '';
  state.script = null;
  state.status = 'planning';
  render();

  const res = await fetch(`${API_BASE}/api/v1/creations`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      user_id: 'web-demo-user',
      prompt: state.prompt,
      idempotency_key: `web-${Date.now()}`,
      assets: parseAssets(state.assetsText),
    }),
  });

  if (!res.ok) {
    state.status = 'failed';
    state.error = await readableError(res);
    render();
    return;
  }

  const data = await res.json();
  state.taskId = data.task_id;
  await pollUntilDone(data.task_id);
}

async function pollUntilDone(id) {
  for (let i = 0; i < 40; i++) {
    const taskRes = await fetch(`${API_BASE}/api/v1/tasks/${id}`);
    const task = await taskRes.json();
    state.status = task.status;
    render();

    if (task.status === 'succeeded') {
      await loadScript(id);
      return;
    }
    if (['failed', 'timeout', 'canceled'].includes(task.status)) {
      state.error = task.error_message || `Task ended with ${task.status}`;
      render();
      return;
    }
    await delay(700);
  }
  state.error = 'Task polling timed out.';
  render();
}

async function loadScript(id = state.taskId) {
  const res = await fetch(`${API_BASE}/api/v1/tasks/${id}/script`);
  if (!res.ok) {
    state.error = await readableError(res);
    render();
    return;
  }
  state.script = await res.json();
  state.error = '';
  render();
}

async function rewrite(kind, index) {
  if (!state.taskId) return;
  const key = `${kind}-${index}`;
  const instruction = state.rewriteText[key] || (kind === 'rewrite-shot' ? 'Make this shot more cinematic and product-focused' : 'Make this voice-over shorter and punchier');
  const res = await fetch(`${API_BASE}/api/v1/tasks/${state.taskId}/${kind}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ shot_index: index, instruction }),
  });
  if (!res.ok) {
    state.error = await readableError(res);
    render();
    return;
  }
  state.script = await res.json();
  state.error = '';
  render();
}

function exportMarkdown() {
  if (state.taskId) window.open(`${API_BASE}/api/v1/tasks/${state.taskId}/script.md`, '_blank');
}

function parseAssets(text) {
  return text.split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const [object_key, kind = 'image'] = line.split(',').map((part) => part.trim());
      return { object_key, kind };
    })
    .filter((asset) => asset.object_key);
}

async function readableError(res) {
  try {
    const body = await res.json();
    return body.message || body.code || res.statusText;
  } catch {
    return res.statusText;
  }
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isBusy() {
  return ['planning', 'pending', 'running'].includes(state.status);
}

function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

