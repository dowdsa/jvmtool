import './style.css';

const App = window.go.main.App;
const PRODUCTS = {
    jdk: { label: 'OpenJDK (Temurin)', short: 'JDK', command: 'java' },
    maven: { label: 'Apache Maven', short: 'Maven', command: 'mvn' },
};
let activeKind = 'jdk';
let downloadQueue = [];
let queueRunning = false;
let currentTaskKey = '';
const queueStorageKey = 'jm.downloadQueue.v1';
let dismissedUpdateVersion = '';
let refreshId = 0;
const app = document.querySelector('#app');

function escapeHTML(value) {
    return String(value).replace(/[&<>'"]/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#039;', '"': '&quot;' }[character]));
}

function render() {
    const product = PRODUCTS[activeKind];
    app.innerHTML = `
        <aside class="sidebar">
            <div class="brand" aria-label="JM version manager"><span class="brand-mark">jm</span><span class="brand-name">version manager</span></div>
            <nav class="product-nav" aria-label="工具类型">
                ${Object.entries(PRODUCTS).map(([kind, item]) => `<button class="nav-item ${kind === activeKind ? 'is-active' : ''}" data-kind="${kind}"><span class="nav-index">0${kind === 'jdk' ? '1' : '2'}</span><span><strong>${item.short}</strong><small>${item.command}</small></span><span class="nav-arrow">↗</span></button>`).join('')}
            </nav>
            <div class="sidebar-controls">
                <div class="sidebar-foot"><span class="live-dot"></span><span>LOCAL ENVIRONMENT</span></div>
                <div class="version-label" id="version-label" title="当前版本">v—</div>
                <button class="settings-btn" id="settings-btn" title="设置">⚙ 设置</button>
            </div>
        </aside>
        <main class="workspace">
            <header class="header"><div class="breadcrumb"><span>TOOLCHAIN</span><i></i><span id="header-kind">${product.short}</span></div><button class="root-path" id="root-path" title="安装根目录">读取工作目录...</button></header>
            <section class="hero">
                <div><p class="eyebrow">ACTIVE TOOLCHAIN</p><h1>${product.short}<span>.</span></h1><p class="hero-copy">${activeKind === 'jdk' ? 'OpenJDK (Temurin) · 管理已安装版本，保持你的本地开发环境井然有序。' : 'Apache Maven · 管理已安装版本，保持你的本地开发环境井然有序。'}</p></div>
                <div class="command-card"><span class="command-label">CURRENT COMMAND</span><code id="current-command">${product.command} —</code></div>
            </section>
            <section class="stats" aria-label="版本概览">
                <div class="stat"><span>INSTALLED</span><strong id="installed-count">—</strong><small>个本地版本</small></div>
                <div class="stat"><span>ACTIVE</span><strong id="active-version">—</strong><small>当前默认版本</small></div>
                <div class="stat stat-note"><span>TIP</span><p>切换版本后，新终端会自动使用该版本。</p></div>
            </section>
            <section class="content-grid">
                <article class="section-card installed-card"><div class="section-heading"><div><p class="section-kicker">YOUR MACHINE</p><h2>已安装版本</h2></div><div class="section-actions"><button class="text-button" id="import-btn">导入目录</button><button class="icon-button" id="refresh-btn" aria-label="刷新已安装版本" title="刷新">↻</button></div></div><div class="version-list" id="installed-list"><div class="empty-state">正在读取本地版本…</div></div></article>
                <article class="section-card discover-card"><div class="section-heading"><div><p class="section-kicker">REMOTE CATALOG</p><h2>发现新版本</h2></div></div><div class="search-box"><label for="search-input">版本号</label><div class="search-row"><input id="search-input" type="text" autocomplete="off" placeholder="例如 17、21 或 3.9" /><button class="button button-dark" id="search-btn">搜索 <span>→</span></button></div><p>留空可浏览全部可用版本</p></div><div class="results-label"><span>AVAILABLE</span><i></i></div><div class="search-results" id="search-results"><div class="empty-state">输入版本号以搜索远程目录。</div></div></article>
            </section>
        </main>
        <section class="download-dock" id="download-dock" hidden aria-live="polite" aria-label="下载任务"></section>`;
    bindEvents();
    renderDownloadQueue();
    loadRoot();
    loadInstalled();
    loadVersion();
    checkUpdateOnStartup();
}

function bindEvents() {
    document.querySelectorAll('[data-kind]').forEach((button) => button.addEventListener('click', () => { activeKind = button.dataset.kind; render(); }));
    document.querySelector('#refresh-btn').addEventListener('click', loadInstalled);
    document.querySelector('#import-btn').addEventListener('click', importInstallation);
    document.querySelector('#search-btn').addEventListener('click', doSearch);
    document.querySelector('#search-input').addEventListener('keydown', (event) => { if (event.key === 'Enter') doSearch(); });
    document.querySelector('#settings-btn').addEventListener('click', openSettings);
}

async function loadVersion() {
    try { const v = await App.GetVersion(); const el = document.querySelector('#version-label'); if (el) el.textContent = 'v' + v; }
    catch (error) { console.error(error); }
}

async function checkUpdateOnStartup() {
    try {
        const info = await App.CheckUpdate();
        if (!info || !info.version) return;
        if (dismissedUpdateVersion === info.version) return;
        const skipped = await App.GetSkipVersion();
        if (skipped === info.version) return; // 用户已跳过此版本
        showUpdateDialog(info);
    } catch (error) { /* 启动时静默失败 */ }
}

async function checkUpdateManual() {
    try {
        const info = await App.CheckUpdate();
        if (info && info.error) { toast(`检查更新失败：${info.error}`, true); return; }
        if (!info || !info.version) {
            toast('当前已是最新版本');
        } else {
            showUpdateDialog(info);
        }
    } catch (error) {
        toast(`检查更新失败：${error}`, true);
    }
}

async function loadRoot() {
    try { const root = await App.Root(); const path = document.querySelector('#root-path'); if (path) path.textContent = root; }
    catch (error) { console.error(error); }
}

async function loadInstalled() {
    const viewId = ++refreshId;
    const kind = activeKind;
    const container = document.querySelector('#installed-list');
    container.innerHTML = '<div class="empty-state"><span class="loader"></span>正在同步本地版本…</div>';
    try {
        const list = await App.List(kind);
        if (viewId !== refreshId || kind !== activeKind) return;
        updateOverview(list || []);
        if (!list || list.length === 0) { container.innerHTML = '<div class="empty-state empty-large"><b>尚无已安装版本</b><span>从右侧目录中搜索并安装一个版本。</span></div>'; return; }
        container.innerHTML = list.map((item, index) => versionRow(item, index)).join('');
        container.querySelectorAll('[data-action]').forEach((button) => button.addEventListener('click', () => { const { action, version } = button.dataset; if (action === 'use') doUse(version); if (action === 'uninstall') doUninstall(version); }));
    } catch (error) { container.innerHTML = `<div class="empty-state error-state">无法读取本地版本：${escapeHTML(error)}</div>`; }
}

function versionRow(item, index) {
    const version = escapeHTML(item.version);
    return `<div class="version-row ${item.current ? 'is-current' : ''}"><span class="row-number">${String(index + 1).padStart(2, '0')}</span><div class="version-name"><strong>${version}</strong><small>${item.current ? 'DEFAULT VERSION' : 'INSTALLED LOCALLY'}</small></div>${item.current ? '<span class="current-tag">CURRENT</span>' : `<button class="text-button" data-action="use" data-version="${version}">设为当前</button>`}<button class="remove-button" data-action="uninstall" data-version="${version}" aria-label="卸载 ${version}" title="卸载">×</button></div>`;
}

function updateOverview(list) {
    const current = list.find((item) => item.current);
    const product = PRODUCTS[activeKind];
    document.querySelector('#installed-count').textContent = list.length;
    document.querySelector('#active-version').textContent = current ? current.version : '—';
    document.querySelector('#current-command').textContent = `${product.command} ${current ? current.version : '—'}`;
}

async function doSearch() {
    const kind = activeKind;
    const query = document.querySelector('#search-input').value.trim();
    const container = document.querySelector('#search-results');
    container.innerHTML = '<div class="empty-state"><span class="loader"></span>正在搜索远程目录…</div>';
    try {
        const versions = await App.Search(kind, query);
        if (kind !== activeKind) return;
        if (!versions || versions.length === 0) { container.innerHTML = '<div class="empty-state">没有找到匹配版本。</div>'; return; }
        container.innerHTML = versions.map((version) => { const safeVersion = escapeHTML(version); return `<div class="result-row"><code>${safeVersion}</code><button class="install-button" data-install="${safeVersion}">加入队列 <span>+</span></button></div>`; }).join('');
        container.querySelectorAll('[data-install]').forEach((button) => button.addEventListener('click', () => enqueueInstall(kind, button.dataset.install)));
    } catch (error) { container.innerHTML = `<div class="empty-state error-state">搜索失败：${escapeHTML(error)}</div>`; }
}

async function doUse(version) { try { await App.Use(activeKind, version); toast(`已切换至 ${PRODUCTS[activeKind].short} ${version}`); loadInstalled(); } catch (error) { toast(`切换失败：${error}`, true); } }
async function doUninstall(version) { if (!confirm(`确定要卸载 ${PRODUCTS[activeKind].short} ${version} 吗？`)) return; try { await App.Uninstall(activeKind, version); toast(`已卸载 ${version}`); loadInstalled(); } catch (error) { toast(`卸载失败：${error}`, true); } }
async function importInstallation() {
    let source;
    try {
        source = await App.SelectDirectory(`选择要导入的 ${PRODUCTS[activeKind].short} 目录`);
    } catch (e) {
        console.error('SelectDirectory failed:', e);
        toast(`打开目录选择器失败：${e}`, true);
        return;
    }
    if (!source) return;
    try {
        const version = await App.Import(activeKind, source, '');
        toast(`已导入 ${PRODUCTS[activeKind].short} ${version}`);
        loadInstalled();
    } catch (error) {
        toast(`导入失败：${error}`, true);
    }
}

function taskKey(kind, version) { return `${kind}:${version}`; }
function getTask(key) { return downloadQueue.find((task) => task.key === key); }

function enqueueInstall(kind, version) {
    const key = taskKey(kind, version);
    if (getTask(key)) { toast(`${version} 已在下载队列中`); return; }
    downloadQueue.push({ key, kind, version, status: 'queued', done: 0, total: 0, rate: 0 });
    renderDownloadQueue();
    runNextDownload();
}

function loadDownloadQueue() {
    try {
        const saved = JSON.parse(localStorage.getItem(queueStorageKey) || '[]');
        if (!Array.isArray(saved)) return;
        downloadQueue = saved.filter((task) => task && task.key && task.kind && task.version)
            .map((task) => ({ ...task, status: task.status === 'downloading' ? 'queued' : task.status }));
    } catch (error) {
        console.warn('无法恢复下载队列', error);
    }
}

function saveDownloadQueue() {
    try {
        localStorage.setItem(queueStorageKey, JSON.stringify(downloadQueue));
    } catch (error) {
        console.warn('无法保存下载队列', error);
    }
}

async function runNextDownload() {
    if (queueRunning) return;
    const task = downloadQueue.find((item) => item.status === 'queued');
    if (!task) { renderDownloadQueue(); return; }
    queueRunning = true;
    currentTaskKey = task.key;
    task.status = 'downloading';
    renderDownloadQueue();
    try {
        const result = await App.Install(task.kind, task.version);
        const status = result && result.status;
        const message = (result && result.message) || '未知错误';
        if (status === 'ok') {
            task.status = 'done';
            toast(`${task.version} 已安装完成`);
            if (task.kind === activeKind) loadInstalled();
        } else if (status === 'paused') {
            task.status = 'paused';
        } else if (status === 'cancelled') {
            task.status = 'cancelled';
            toast(`${task.version} 已取消下载`, true);
        } else {
            task.status = 'error';
            task.message = message;
            toast(`${task.version} 安装失败：${message}`, true);
        }
    } catch (error) {
        task.status = 'error';
        task.message = String(error);
        toast(`${task.version} 安装失败：${error}`, true);
    } finally {
        queueRunning = false;
        currentTaskKey = '';
        renderDownloadQueue();
        if (task.status === 'done' || task.status === 'cancelled' || task.status === 'error') {
            downloadQueue = downloadQueue.filter((item) => item !== task);
            renderDownloadQueue();
            runNextDownload();
        }
    }
}

// ---------- 下载进度（内嵌） ----------
function formatSize(bytes) {
    if (!bytes || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    let value = bytes, index = 0;
    while (value >= 1024 && index < units.length - 1) { value /= 1024; index++; }
    return `${value.toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
}

function formatRate(rate) {
    if (!rate || rate <= 0) return '—';
    return `${formatSize(rate)}/s`;
}

function renderDownloadQueue() {
    const container = document.querySelector('#download-dock');
    if (!container) return;
    saveDownloadQueue();
    const visible = downloadQueue.filter((task) => task.status !== 'done' && task.status !== 'cancelled');
    container.hidden = visible.length === 0;
    container.innerHTML = visible.map((task) => progressRow(task)).join('');
    container.querySelectorAll('[data-queue-act]').forEach((button) => button.addEventListener('click', () => handleQueueAction(button.dataset.queueAct, button.dataset.queueKey)));
}

function progressRow(task) {
    const { kind, version, done, total, rate, status, message } = task;
    const short = PRODUCTS[kind]?.short || kind;
    const safeKey = escapeHTML(task.key);
    const safeVersion = escapeHTML(version);
    const pct = total > 0 ? Math.min(100, Math.round((done / total) * 100)) : 0;
    const stateText = status === 'error' ? '下载失败' : status === 'paused' ? '已暂停' : status === 'queued' ? '排队中' : '下载中';
    const action = status === 'queued'
        ? `<button class="progress-btn progress-btn-cancel" data-queue-act="remove" data-queue-key="${safeKey}">移除</button>`
        : status === 'paused'
            ? `<button class="progress-btn" data-queue-act="resume" data-queue-key="${safeKey}">继续</button><button class="progress-btn progress-btn-cancel" data-queue-act="cancel" data-queue-key="${safeKey}">取消</button>`
            : status === 'error'
                ? `<button class="progress-btn" data-queue-act="retry" data-queue-key="${safeKey}">重试</button><button class="progress-btn progress-btn-cancel" data-queue-act="remove" data-queue-key="${safeKey}">移除</button>`
                : `<button class="progress-btn" data-queue-act="pause" data-queue-key="${safeKey}">暂停</button><button class="progress-btn progress-btn-cancel" data-queue-act="cancel" data-queue-key="${safeKey}">取消</button>`;
    return `
        <div class="progress-row" data-progress-key="${safeKey}">
        <div class="progress-info">
            <div class="progress-title-row">
                <span class="progress-title">${short} ${safeVersion}</span>
                <span class="progress-percent">${status === 'queued' ? '—' : pct + '%'}</span>
            </div>
            <div class="progress-track"><div class="progress-fill" style="width:${pct}%"></div></div>
            <div class="progress-meta">
                <span>${stateText}</span>
                <span>${formatSize(done)} / ${formatSize(total)}</span>
                <span>${status === 'error' ? escapeHTML(message || '') : formatRate(rate)}</span>
            </div>
        </div>
        <div class="progress-actions">${action}</div></div>`;
}

function handleQueueAction(action, key) {
    const task = getTask(key);
    if (!task) return;
    if (action === 'pause' && key === currentTaskKey) App.PauseInstall();
    if (action === 'cancel' && key === currentTaskKey) App.CancelInstall();
    if (action === 'remove' || action === 'cancel' && key !== currentTaskKey) {
        downloadQueue = downloadQueue.filter((item) => item !== task);
        renderDownloadQueue();
    }
    if (action === 'resume' || action === 'retry') {
        task.status = 'queued';
        task.message = '';
        renderDownloadQueue();
        runNextDownload();
    }
}

function bindProgressEvents() {
    window.runtime.EventsOn('install:progress', (payload) => {
        if (!payload) return;
        const task = getTask(taskKey(payload.kind, payload.version));
        if (!task) return;
        task.done = payload.done || task.done || 0;
        task.total = payload.total || task.total || 0;
        task.rate = payload.rate || 0;
        if (payload.status === 'paused') task.status = 'paused';
        if (payload.status === 'error') { task.status = 'error'; task.message = payload.message || ''; }
        if (payload.status === 'cancelled') task.status = 'cancelled';
        renderDownloadQueue();
    });
}

function toast(message, isError = false) {
    const toastElement = document.createElement('div');
    toastElement.className = `toast ${isError ? 'is-error' : ''}`;
    toastElement.textContent = message;
    document.body.appendChild(toastElement);
    requestAnimationFrame(() => toastElement.classList.add('is-visible'));
    setTimeout(() => { toastElement.classList.remove('is-visible'); setTimeout(() => toastElement.remove(), 180); }, 3400);
}

// ---------- 设置 ----------
async function openSettings() {
    let proxy = '';
    try { proxy = await App.GetProxy(); } catch (e) { /* ignore */ }
    let ver = '';
    try { ver = await App.GetVersion(); } catch (e) { /* ignore */ }
    let autoStart = false;
    try { autoStart = await App.GetAutoStart(); } catch (e) { /* ignore */ }
    let closeBehavior = 'ask';
    try { closeBehavior = await App.GetCloseBehavior(); } catch (e) { /* ignore */ }

    const overlay = document.createElement('div');
    overlay.id = 'settings-overlay';
    overlay.className = 'settings-overlay';
    overlay.innerHTML = `
        <div class="settings-card">
            <div class="settings-head">
                <h2>设置</h2>
                <button class="settings-close" id="settings-close">×</button>
            </div>
            <div class="settings-body">
                <label class="settings-label" for="proxy-input">代理地址 (Proxy)</label>
                <input class="settings-input" id="proxy-input" type="text"
                       placeholder="例如 http://127.0.0.1:7890 或 socks5://127.0.0.1:1080"
                       value="${escapeHTML(proxy)}" autocomplete="off" />
                <p class="settings-hint">支持 http / https / socks5 协议。留空表示不使用代理（直连）。<br>设置后立即生效，无需重启。</p>
                <label class="settings-label" for="close-behavior-input">关闭窗口行为</label>
                <select class="settings-input" id="close-behavior-input">
                    <option value="ask" ${closeBehavior === 'ask' ? 'selected' : ''}>每次询问</option>
                    <option value="tray" ${closeBehavior === 'tray' ? 'selected' : ''}>最小化到后台</option>
                    <option value="quit" ${closeBehavior === 'quit' ? 'selected' : ''}>直接退出</option>
                </select>
                <label class="settings-check"><input id="autostart-input" type="checkbox" ${autoStart ? 'checked' : ''} /> <span>开机自启动</span></label>
                <div class="settings-about">
                    <span class="settings-about-ver">当前版本 v${escapeHTML(ver)}</span>
                    <button class="button button-dark" id="check-update-btn">检查更新</button>
                </div>
            </div>
            <div class="settings-foot">
                <button class="button button-dark" id="settings-save">保存</button>
            </div>
        </div>`;
    document.body.appendChild(overlay);

    const close = () => overlay.remove();
    overlay.querySelector('#settings-close').addEventListener('click', close);
    overlay.addEventListener('click', (e) => { if (e.target === overlay) close(); });
    overlay.querySelector('#settings-save').addEventListener('click', async () => {
        const value = overlay.querySelector('#proxy-input').value.trim();
        const cb = overlay.querySelector('#close-behavior-input').value;
        try {
            await App.SetProxy(value);
            await App.SetAutoStart(overlay.querySelector('#autostart-input').checked);
            await App.SetCloseBehavior(cb);
            close();
            toast('设置已保存');
        } catch (error) {
            toast(`保存失败：${error}`, true);
        }
    });
    overlay.querySelector('#check-update-btn').addEventListener('click', async () => {
        close();
        await checkUpdateManual();
    });
    overlay.querySelector('#proxy-input').focus();
}

// ---------- 更新弹窗 ----------
function showUpdateDialog(info) {
    if (document.querySelector('#update-overlay')) return;
    const overlay = document.createElement('div');
    overlay.id = 'update-overlay';
    overlay.className = 'settings-overlay';
    overlay.innerHTML = `
        <div class="settings-card update-card">
            <div class="settings-head">
                <h2>发现新版本</h2>
            </div>
            <div class="settings-body">
                <p class="update-copy">jm 有新版本 <strong>v${escapeHTML(info.version)}</strong> 可用，建议更新。</p>
            </div>
            <div class="settings-foot update-foot">
                <button class="update-skip" id="update-skip">跳过此版本</button>
                <div class="update-actions">
                    <button class="update-cancel" id="update-cancel">取消</button>
                    <button class="button button-dark" id="update-install">安装</button>
                </div>
            </div>
        </div>`;
    document.body.appendChild(overlay);

    const close = () => overlay.remove();
    overlay.querySelector('#update-cancel').addEventListener('click', () => {
        dismissedUpdateVersion = info.version;
        close();
    });
    overlay.addEventListener('click', (e) => {
        if (e.target === overlay) {
            dismissedUpdateVersion = info.version;
            close();
        }
    });
    overlay.querySelector('#update-skip').addEventListener('click', async () => {
        dismissedUpdateVersion = info.version;
        try { await App.SkipVersion(info.version); } catch (e) { /* ignore */ }
        close();
    });
    overlay.querySelector('#update-install').addEventListener('click', () => {
        const button = overlay.querySelector('#update-install');
        button.disabled = true;
        button.textContent = '正在下载…';
        App.InstallUpdate().then(() => {
            button.textContent = '正在启动安装程序…';
        }).catch((error) => {
            button.disabled = false;
            button.textContent = '安装更新';
            toast(`更新失败：${error}`, true);
        });
    });
}

// ---------- 关闭确认弹窗 ----------
function showCloseDialog() {
    if (document.querySelector('#close-overlay')) return;
    const overlay = document.createElement('div');
    overlay.id = 'close-overlay';
    overlay.className = 'settings-overlay';
    overlay.innerHTML = `
        <div class="settings-card" style="max-width:400px">
            <div class="settings-head">
                <h2>关闭窗口</h2>
            </div>
            <div class="settings-body">
                <p style="margin:0;color:var(--text-secondary,#94a3b8)">关闭窗口后，jm 可以在后台继续运行，方便下次快速使用。</p>
            </div>
            <div style="display:flex;justify-content:space-between;align-items:center;padding:16px 24px 20px">
                <label class="settings-check" style="margin:0">
                    <input id="close-remember" type="checkbox" />
                    <span>不再提示，记住我的选择</span>
                </label>
                <div style="display:flex;gap:8px">
                    <button class="update-cancel" id="close-quit-btn">退出应用</button>
                    <button class="button button-dark" id="close-tray-btn">后台运行</button>
                </div>
            </div>
        </div>`;
    document.body.appendChild(overlay);

    overlay.querySelector('#close-tray-btn').addEventListener('click', () => {
        const remember = overlay.querySelector('#close-remember').checked;
        overlay.remove();
        App.ConfirmClose(true, remember);
    });
    overlay.querySelector('#close-quit-btn').addEventListener('click', () => {
        const remember = overlay.querySelector('#close-remember').checked;
        overlay.remove();
        App.ConfirmClose(false, remember);
    });
}

// Listen for close:confirm event from backend
window.runtime.EventsOn('close:confirm', () => {
    showCloseDialog();
});

bindProgressEvents();
loadDownloadQueue();
render();
setTimeout(runNextDownload, 250);
