import './style.css';

const App = window['go']['main']['App'];

const KINDS = {
    jdk: 'JDK',
    maven: 'Maven',
};

let activeKind = 'jdk';
let searchResults = [];
let installing = false;

// ---------- 视图渲染 ----------
const app = document.querySelector('#app');

function render() {
    app.innerHTML = `
        <div class="topbar">
            <div class="title">jm - JDK & Maven 版本管理</div>
            <div class="root" id="root-path">加载中...</div>
        </div>
        <div class="tabs">
            <div class="tab ${activeKind === 'jdk' ? 'active' : ''}" data-kind="jdk">JDK</div>
            <div class="tab ${activeKind === 'maven' ? 'active' : ''}" data-kind="maven">Maven</div>
        </div>
        <div class="content">
            <div class="panel">
                <div class="panel-title">
                    <span>已安装的 ${KINDS[activeKind]}</span>
                    <button class="btn-ghost" id="refresh-btn">刷新</button>
                </div>
                <div class="panel-body" id="installed-list">
                    <div class="empty">加载中...</div>
                </div>
            </div>
            <div class="panel">
                <div class="panel-title"><span>搜索 & 安装</span></div>
                <div class="panel-body">
                    <div class="search-row">
                        <input id="search-input" type="text"
                               placeholder="输入版本关键字，如 17 / 3.9（留空列出全部）" />
                        <button class="btn-primary" id="search-btn">搜索</button>
                    </div>
                    <div class="search-results" id="search-results">
                        <div class="empty">输入关键字搜索远程可用版本</div>
                    </div>
                </div>
            </div>
        </div>
    `;

    bindEvents();
    loadInstalled();
    loadRoot();
}

// ---------- 事件绑定 ----------
function bindEvents() {
    document.querySelectorAll('.tab').forEach((el) => {
        el.addEventListener('click', () => {
            activeKind = el.dataset.kind;
            render();
        });
    });

    document.getElementById('refresh-btn').addEventListener('click', loadInstalled);

    document.getElementById('search-btn').addEventListener('click', doSearch);
    document.getElementById('search-input').addEventListener('keydown', (e) => {
        if (e.key === 'Enter') doSearch();
    });
}

// ---------- 数据加载 ----------
async function loadRoot() {
    try {
        const root = await App.Root();
        document.getElementById('root-path').textContent = `根目录: ${root}`;
    } catch (e) {
        console.error(e);
    }
}

async function loadInstalled() {
    const container = document.getElementById('installed-list');
    try {
        const list = await App.List(activeKind);
        if (!list || list.length === 0) {
            container.innerHTML = `<div class="empty">尚未安装任何 ${KINDS[activeKind]} 版本</div>`;
            return;
        }
        container.innerHTML = list.map((v) => `
            <div class="list-item">
                <div>
                    <span class="ver">${v.version}</span>
                    ${v.current ? '<span class="badge">当前</span>' : ''}
                </div>
                <div class="actions">
                    ${v.current ? '' : `<button class="btn-primary" data-act="use" data-ver="${v.version}">切换</button>`}
                    <button class="btn-danger" data-act="uninstall" data-ver="${v.version}">卸载</button>
                </div>
            </div>
        `).join('');

        container.querySelectorAll('button[data-act]').forEach((btn) => {
            btn.addEventListener('click', async () => {
                const act = btn.dataset.act;
                const ver = btn.dataset.ver;
                if (act === 'use') {
                    await doUse(ver);
                } else if (act === 'uninstall') {
                    await doUninstall(ver);
                }
            });
        });
    } catch (e) {
        container.innerHTML = `<div class="empty">加载失败: ${e}</div>`;
    }
}

async function doSearch() {
    const input = document.getElementById('search-input');
    const query = input.value.trim();
    const container = document.getElementById('search-results');
    container.innerHTML = `<div class="empty"><span class="spinner"></span>搜索中...</div>`;
    try {
        searchResults = await App.Search(activeKind, query);
        if (!searchResults || searchResults.length === 0) {
            container.innerHTML = `<div class="empty">没有匹配的版本</div>`;
            return;
        }
        container.innerHTML = searchResults.map((v) => `
            <div class="result-item">
                <span class="ver">${v}</span>
                <button class="btn-primary" data-install="${v}">安装</button>
            </div>
        `).join('');

        container.querySelectorAll('button[data-install]').forEach((btn) => {
            btn.addEventListener('click', async () => {
                await doInstall(btn.dataset.install);
            });
        });
    } catch (e) {
        container.innerHTML = `<div class="empty">搜索失败: ${e}</div>`;
    }
}

// ---------- 操作 ----------
async function doUse(version) {
    try {
        await App.Use(activeKind, version);
        toast(`已切换到 ${KINDS[activeKind]} ${version}`);
        loadInstalled();
    } catch (e) {
        toast(`切换失败: ${e}`, true);
    }
}

async function doUninstall(version) {
    if (!confirm(`确认卸载 ${KINDS[activeKind]} ${version}？`)) return;
    try {
        await App.Uninstall(activeKind, version);
        toast(`已卸载 ${KINDS[activeKind]} ${version}`);
        loadInstalled();
    } catch (e) {
        toast(`卸载失败: ${e}`, true);
    }
}

async function doInstall(version) {
    if (installing) return;
    installing = true;
    toast(`开始安装 ${KINDS[activeKind]} ${version}，请稍候...`);
    try {
        await App.Install(activeKind, version);
        toast(`安装成功: ${KINDS[activeKind]} ${version}`);
        loadInstalled();
    } catch (e) {
        toast(`安装失败: ${e}`, true);
    } finally {
        installing = false;
    }
}

// ---------- 提示 ----------
function toast(msg, isError = false) {
    const el = document.createElement('div');
    el.className = 'toast' + (isError ? ' error' : '');
    el.textContent = msg;
    document.body.appendChild(el);
    setTimeout(() => el.remove(), 4000);
}

render();
