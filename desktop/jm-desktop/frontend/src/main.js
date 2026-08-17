import './style.css';

const App = window.go.main.App;
const PRODUCTS = {
    jdk: { label: 'OpenJDK (Temurin)', short: 'JDK', command: 'java' },
    maven: { label: 'Apache Maven', short: 'Maven', command: 'mvn' },
};
let activeKind = 'jdk';
let installing = false;
let paused = false;
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
                <article class="section-card installed-card"><div class="section-heading"><div><p class="section-kicker">YOUR MACHINE</p><h2>已安装版本</h2></div><button class="icon-button" id="refresh-btn" aria-label="刷新已安装版本" title="刷新">↻</button></div><div class="version-list" id="installed-list"><div class="empty-state">正在读取本地版本…</div></div></article>
                <article class="section-card discover-card"><div class="section-heading"><div><p class="section-kicker">REMOTE CATALOG</p><h2>发现新版本</h2></div></div><div class="search-box"><label for="search-input">版本号</label><div class="search-row"><input id="search-input" type="text" autocomplete="off" placeholder="例如 17、21 或 3.9" /><button class="button button-dark" id="search-btn">搜索 <span>→</span></button></div><p>留空可浏览全部可用版本</p></div><div class="results-label"><span>AVAILABLE</span><i></i></div><div class="search-results" id="search-results"><div class="empty-state">输入版本号以搜索远程目录。</div></div></article>
            </section>
        </main>
        <section class="download-dock" id="download-dock" hidden aria-live="polite" aria-label="下载任务"></section>`;
    bindEvents();
    loadRoot();
    loadInstalled();
    loadVersion();
    checkUpdateOnStartup();
}

function bindEvents() {
    document.querySelectorAll('[data-kind]').forEach((button) => button.addEventListener('click', () => { activeKind = button.dataset.kind; render(); }));
    document.querySelector('#refresh-btn').addEventListener('click', loadInstalled);
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
        const skipped = await App.GetSkipVersion();
        if (skipped === info.version) return; // 用户已跳过此版本
        showUpdateDialog(info);
    } catch (error) { /* 启动时静默失败 */ }
}

async function checkUpdateManual() {
    try {
        const info = await App.CheckUpdate();
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
        container.innerHTML = versions.map((version) => { const safeVersion = escapeHTML(version); return `<div class="result-row"><code>${safeVersion}</code><button class="install-button" data-install="${safeVersion}">安装 <span>+</span></button></div>`; }).join('');
        container.querySelectorAll('[data-install]').forEach((button) => button.addEventListener('click', () => doInstall(button.dataset.install)));
    } catch (error) { container.innerHTML = `<div class="empty-state error-state">搜索失败：${escapeHTML(error)}</div>`; }
}

async function doUse(version) { try { await App.Use(activeKind, version); toast(`已切换至 ${PRODUCTS[activeKind].short} ${version}`); loadInstalled(); } catch (error) { toast(`切换失败：${error}`, true); } }
async function doUninstall(version) { if (!confirm(`确定要卸载 ${PRODUCTS[activeKind].short} ${version} 吗？`)) return; try { await App.Uninstall(activeKind, version); toast(`已卸载 ${version}`); loadInstalled(); } catch (error) { toast(`卸载失败：${error}`, true); } }

function removeProgressRow(version) {
    const row = document.querySelector(`#download-dock [data-progress-version="${version}"]`);
    const dock = document.querySelector('#download-dock');
    if (row) row.remove();
    if (dock && dock.children.length === 0) dock.hidden = true;
}

async function doInstall(version) {
    if (installing) return;
    installing = true;
    paused = false;
    const kind = activeKind;
    renderProgress(kind, version, 0, 0, 0, 'downloading');
    try {
        const result = await App.Install(kind, version);
        const status = result && result.status;
        const message = (result && result.message) || '未知错误';
        if (status === 'ok') {
            removeProgressRow(version);
            toast(`${version} 已安装完成`);
            loadInstalled();
        } else if (status === 'paused') {
            // 暂停：保留进度行，更新为已暂停状态
            markProgressPaused(version);
        } else if (status === 'cancelled') {
            removeProgressRow(version);
            toast('已取消下载', true);
        } else {
            const row = document.querySelector(`[data-progress-version="${version}"]`);
            if (row) updateProgressRow(row, kind, version, 0, 0, 0, 'error');
            toast(`安装失败：${message}`, true);
        }
    } catch (error) {
        const row = document.querySelector(`[data-progress-version="${version}"]`);
        if (row) updateProgressRow(row, kind, version, 0, 0, 0, 'error');
        toast(`安装失败：${error}`, true);
    } finally {
        installing = false;
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

function renderProgress(kind, version, done, total, rate, status) {
    const container = document.querySelector('#download-dock');
    if (!container) return;
    container.hidden = false;
    let row = container.querySelector(`[data-progress-version="${version}"]`);
    if (!row) {
        row = document.createElement('div');
        row.className = 'progress-row';
        row.dataset.progressVersion = version;
        container.prepend(row);
    }
    updateProgressRow(row, kind, version, done, total, rate, status);
}

function markProgressPaused(version) {
    const row = document.querySelector(`[data-progress-version="${version}"]`);
    if (!row) return;
    const done = parseInt(row.dataset.done || '0', 10);
    const total = parseInt(row.dataset.total || '0', 10);
    updateProgressRow(row, activeKind, version, done, total, 0, 'paused');
}

function updateProgressRow(row, kind, version, done, total, rate, status) {
    row.dataset.done = String(done || 0);
    row.dataset.total = String(total || 0);
    const short = PRODUCTS[kind]?.short || kind;
    const pct = total > 0 ? Math.min(100, Math.round((done / total) * 100)) : 0;
    const stateText = status === 'error' ? '下载失败'
        : status === 'cancelled' ? '已取消'
        : status === 'paused' ? '已暂停'
        : '下载中';
    row.innerHTML = `
        <div class="progress-info">
            <div class="progress-title-row">
                <span class="progress-title">${short} ${version}</span>
                <span class="progress-percent">${status === 'paused' ? '' : pct + '%'}</span>
            </div>
            <div class="progress-track"><div class="progress-fill" style="width:${pct}%"></div></div>
            <div class="progress-meta">
                <span>${stateText}</span>
                <span>${formatSize(done)} / ${formatSize(total)}</span>
                <span>${status === 'paused' ? '' : formatRate(rate)}</span>
            </div>
        </div>
        <div class="progress-actions">
            ${status === 'error' || status === 'cancelled' ? '' : status === 'paused'
                ? `<button class="progress-btn" data-progress-act="resume">继续</button>`
                : `<button class="progress-btn" data-progress-act="pause">暂停</button>`}
            ${status === 'error' || status === 'cancelled' ? ''
                : `<button class="progress-btn progress-btn-cancel" data-progress-act="cancel">取消</button>`}
        </div>`;
    bindProgressActions(row);
}

function bindProgressActions(row) {
    row.querySelectorAll('[data-progress-act]').forEach((button) => button.addEventListener('click', () => {
        const act = button.dataset.progressAct;
        if (act === 'pause') {
            paused = true;
            App.PauseInstall();
        } else if (act === 'resume') {
            paused = false;
            const version = row.dataset.progressVersion;
            row.remove();
            doInstall(version);
        } else if (act === 'cancel') {
            App.CancelInstall();
        }
    }));
}

function bindProgressEvents() {
    window.runtime.EventsOn('install:progress', (payload) => {
        if (!payload) return;
        if (payload.status === 'error' || payload.status === 'cancelled') {
            const row = document.querySelector(`[data-progress-version="${payload.version}"]`);
            if (row) updateProgressRow(row, payload.kind, payload.version, payload.done || 0, payload.total || 0, 0, payload.status);
            return;
        }
        renderProgress(payload.kind, payload.version, payload.done, payload.total, payload.rate, payload.status);
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
        try {
            await App.SetProxy(value);
            close();
            toast(value ? '代理已保存并生效' : '代理已清除');
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
    overlay.querySelector('#update-cancel').addEventListener('click', close);
    overlay.addEventListener('click', (e) => { if (e.target === overlay) close(); });
    overlay.querySelector('#update-skip').addEventListener('click', async () => {
        try { await App.SkipVersion(info.version); } catch (e) { /* ignore */ }
        close();
    });
    overlay.querySelector('#update-install').addEventListener('click', () => {
        if (info.url) window.runtime.BrowserOpenURL(info.url);
        close();
    });
}

bindProgressEvents();
render();
