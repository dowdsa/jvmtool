import './style.css';

const App = window.go.main.App;
const PRODUCTS = {
    jdk: { label: 'Java Development Kit', short: 'JDK', command: 'java' },
    maven: { label: 'Apache Maven', short: 'Maven', command: 'mvn' },
};
let activeKind = 'jdk';
let installing = false;
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
            <div class="sidebar-foot"><span class="live-dot"></span><span>LOCAL ENVIRONMENT</span></div>
        </aside>
        <main class="workspace">
            <header class="header"><div class="breadcrumb"><span>TOOLCHAIN</span><i></i><span id="header-kind">${product.short}</span></div><button class="root-path" id="root-path" title="安装根目录">读取工作目录...</button></header>
            <section class="hero">
                <div><p class="eyebrow">ACTIVE TOOLCHAIN</p><h1>${product.short}<span>.</span></h1><p class="hero-copy">管理已安装版本，保持你的本地开发环境井然有序。</p></div>
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
        </main>`;
    bindEvents();
    loadRoot();
    loadInstalled();
}

function bindEvents() {
    document.querySelectorAll('[data-kind]').forEach((button) => button.addEventListener('click', () => { activeKind = button.dataset.kind; render(); }));
    document.querySelector('#refresh-btn').addEventListener('click', loadInstalled);
    document.querySelector('#search-btn').addEventListener('click', doSearch);
    document.querySelector('#search-input').addEventListener('keydown', (event) => { if (event.key === 'Enter') doSearch(); });
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
async function doInstall(version) { if (installing) return; installing = true; toast(`正在安装 ${PRODUCTS[activeKind].short} ${version}…`); try { await App.Install(activeKind, version); toast(`${version} 已安装完成`); loadInstalled(); } catch (error) { toast(`安装失败：${error}`, true); } finally { installing = false; } }

function toast(message, isError = false) {
    const toastElement = document.createElement('div');
    toastElement.className = `toast ${isError ? 'is-error' : ''}`;
    toastElement.textContent = message;
    document.body.appendChild(toastElement);
    requestAnimationFrame(() => toastElement.classList.add('is-visible'));
    setTimeout(() => { toastElement.classList.remove('is-visible'); setTimeout(() => toastElement.remove(), 180); }, 3400);
}

render();
