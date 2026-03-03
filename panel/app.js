/* ============================================================
   ts-panel app.js — SPA 路由 + API + 交互逻辑
   ============================================================ */

// === 状态 ===
const state = {
    token: localStorage.getItem('ts_admin_token') || '',
    activeTab: 'checkout',
};

// === DOM 工具 ===
const $ = id => document.getElementById(id);
const show = el => { if (el) el.classList.remove('hide'); };
const hide = el => { if (el) el.classList.add('hide'); };
const toggle = (el, v) => v ? show(el) : hide(el);

// === 视图切换 ===
function showView(viewId) {
    $('view-login').classList.toggle('hide', viewId !== 'login');
    $('view-main').classList.toggle('hide', viewId !== 'main');
}

function switchTab(tab) {
    state.activeTab = tab;
    $('tab-checkout-content').style.display = tab === 'checkout' ? 'block' : 'none';
    $('tab-instances-content').style.display = tab === 'instances' ? 'block' : 'none';
    $('tab-checkout').classList.toggle('active', tab === 'checkout');
    $('tab-instances').classList.toggle('active', tab === 'instances');
    if (tab === 'instances') loadInstances();
}

// === API ===
async function api(method, path, body) {
    const res = await fetch('/api' + path, {
        method,
        headers: {
            'Content-Type': 'application/json',
            'X-Admin-Token': state.token,
        },
        body: body ? JSON.stringify(body) : undefined,
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data?.error?.message || `HTTP ${res.status}`);
    return data;
}

// === Toast ===
let toastTimer;
function showToast(msg, duration = 3000) {
    const t = $('toast');
    t.textContent = msg;
    show(t);
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => hide(t), duration);
}

// === 确认弹窗 ===
function confirm(title, body) {
    return new Promise(resolve => {
        $('modal-title').textContent = title;
        $('modal-body').textContent = body;
        show($('confirm-modal'));
        const cleanup = (result) => {
            hide($('confirm-modal'));
            resolve(result);
        };
        $('modal-confirm').onclick = () => cleanup(true);
        $('modal-cancel').onclick = () => cleanup(false);
    });
}

// === 登录 ===
$('login-form').addEventListener('submit', async e => {
    e.preventDefault();
    const token = $('admin-token').value.trim();
    if (!token) return;
    state.token = token;
    // 验证 token
    try {
        await api('GET', '/instances');
        localStorage.setItem('ts_admin_token', token);
        hide($('login-error'));
        showView('main');
    } catch {
        show($('login-error'));
        $('login-error').textContent = 'Token 无效，请重新输入';
        state.token = '';
    }
});

$('logout-btn').addEventListener('click', () => {
    localStorage.removeItem('ts_admin_token');
    state.token = '';
    showView('login');
    $('admin-token').value = '';
});

// === 标签切换 ===
$('tab-checkout').addEventListener('click', () => switchTab('checkout'));
$('tab-instances').addEventListener('click', () => switchTab('instances'));

// === Checkout 表单 ===
$('checkout-form').addEventListener('submit', async e => {
    e.preventDefault();
    const spinner = $('checkout-spinner');
    const btnText = $('checkout-btn-text');
    show(spinner);
    btnText.textContent = '处理中…';
    $('checkout-btn').disabled = true;

    try {
        const req = {
            platform: $('platform').value,
            platform_user: $('platform-user').value.trim(),
            order_no: $('order-no').value.trim() || undefined,
            note: $('note').value.trim() || undefined,
            slots: parseInt($('slots').value) || 15,
            duration: $('duration').value || undefined,
            reuse_recycled: $('reuse-recycled').checked,
        };

        const resp = await api('POST', '/instances/checkout', req);
        showDeliveryResult(resp);
    } catch (err) {
        showToast('❌ ' + err.message, 5000);
    } finally {
        hide(spinner);
        btnText.textContent = '创建并发货';
        $('checkout-btn').disabled = false;
    }
});

function renderSecretsBlock(secrets) {
    if (!secrets) return '';
    // 字段来源兼容 checkout resp (snake_case) 和 instance obj
    const loginName = secrets.login_name || '';
    const adminPass = secrets.admin_password || '';
    const apiKey = secrets.api_key || '';
    const privKey = secrets.privilege_key || secrets.admin_privilege_key || '';
    const queryPass = secrets.query_password || secrets.serverquery_password || '';

    if (!loginName && !adminPass && !apiKey && !privKey && !queryPass) return '';

    const rows = [
        loginName ? ['账号名称 (loginname)', loginName] : null,
        adminPass ? ['管理员密码 (password)', adminPass] : null,
        apiKey ? ['API 密钥 (apikey)', apiKey] : null,
        privKey ? ['特权令牌 (token)', privKey] : null,
        queryPass ? ['Query 密码', queryPass] : null,
    ].filter(Boolean);

    return `
    <div class="secrets-block">
      <div class="secrets-title">🔑 服务器密钥（请妥善保管）</div>
      ${rows.map(([label, val]) => `<div class="secret-row">
        <span class="secret-label">${label}</span>
        <code class="secret-value">${escapeHtml(val)}</code>
        <button class="btn btn-ghost btn-xs" onclick="copyToClipboard('${escapeHtml(val)}')">Copy</button>
      </div>`).join('')}
    </div>`;
}

function showDeliveryResult(resp) {
    const card = $('delivery-result');
    show(card);
    $('delivery-text').textContent = resp.delivery_text || '';

    // 密鑰区块
    const secretsEl = $('delivery-secrets');
    if (secretsEl) {
        secretsEl.innerHTML = renderSecretsBlock(resp.secrets);
        toggle(secretsEl, !!(resp.secrets));
    }

    const warnContainer = $('warnings-container');
    if (resp.warnings && resp.warnings.length > 0) {
        warnContainer.innerHTML = resp.warnings.map(w =>
            `<div class="warning-item">⚠️ ${escapeHtml(w)}</div>`
        ).join('');
        show(warnContainer);
    } else {
        hide(warnContainer);
    }

    card.scrollIntoView({ behavior: 'smooth', block: 'center' });
}


// === 复制 & 分享 ===
$('copy-btn').addEventListener('click', () => {
    const text = $('delivery-text').textContent;
    copyToClipboard(text);
});

$('share-btn').addEventListener('click', () => {
    const text = $('delivery-text').textContent;
    if (navigator.share) {
        navigator.share({ text }).catch(() => copyToClipboard(text));
    } else {
        copyToClipboard(text);
    }
});

async function copyToClipboard(text) {
    try {
        await navigator.clipboard.writeText(text);
        showToast('✅ 已复制到剪贴板');
    } catch {
        // fallback: hidden textarea
        const ta = document.createElement('textarea');
        ta.value = text;
        ta.style.cssText = 'position:fixed;left:-9999px;top:-9999px;opacity:0';
        document.body.appendChild(ta);
        ta.select();
        try {
            document.execCommand('copy');
            showToast('✅ 已复制到剪贴板');
        } catch {
            showToast('❌ 复制失败，请手动复制');
        }
        document.body.removeChild(ta);
    }
}

// === 实例列表 ===
$('refresh-btn').addEventListener('click', loadInstances);

async function loadInstances() {
    show($('instances-loading'));
    hide($('instances-empty'));
    hide($('instances-list'));

    try {
        const data = await api('GET', '/instances');
        const instances = data.instances || [];
        hide($('instances-loading'));

        if (instances.length === 0) {
            show($('instances-empty'));
            return;
        }
        show($('instances-list'));
        $('instances-list').innerHTML = instances.map(renderInstance).join('');
        // 绑定按钮事件
        bindInstanceActions();
    } catch (err) {
        hide($('instances-loading'));
        showToast('❌ 加载失败: ' + err.message);
    }
}

function statusLabel(status) {
    const map = { running: '运行中', stopped: '已停止', recycled: '已回收', creating: '创建中', failed: '失败' };
    return map[status] || status;
}

function renderInstance(inst) {
    const secretsHtml = renderSecretsBlock(inst);
    return `
  <div class="instance-card" data-id="${inst.id}">
    <div class="instance-header">
      <span class="instance-name">${escapeHtml(inst.container_name)}</span>
      <span class="status-badge status-${inst.status}">${statusLabel(inst.status)}</span>
    </div>
    <div class="instance-meta">
      <span class="meta-tag">UDP :${inst.host_udp_port}</span>
      <span class="meta-tag">Slots ${inst.slots}${inst.slots_applied ? ' ✓' : ' ⚠'}</span>
      ${inst.customer_id ? `<span class="meta-tag">CID …${inst.customer_id.slice(-6)}</span>` : '<span class="meta-tag">无客户</span>'}
      <span class="meta-tag">创建 ${formatTime(inst.created_at)}</span>
      ${inst.expires_at ? `<span class="meta-tag ${isExpired(inst.expires_at) ? 'meta-tag-expired' : ''}">到期 ${formatTime(inst.expires_at)}</span>` : '<span class="meta-tag">永久</span>'}
    </div>
    ${inst.last_delivery_text ? `<div class="instance-delivery">${escapeHtml(inst.last_delivery_text)}</div>` : ''}
    ${secretsHtml}
    <div class="instance-actions">
        <button class="btn btn-ghost btn-sm" data-action="copy-delivery" data-id="${inst.id}" data-text="${encodeURIComponent(inst.last_delivery_text || '')}">
            复制文本
        </button>
        <button class="btn btn-ghost btn-sm" data-action="logs" data-id="${inst.id}" data-name="${escapeHtml(inst.container_name)}">📋 日志</button>
        <button class="btn btn-ghost btn-sm" data-action="backup" data-id="${inst.id}">💾 备份</button>
        <button class="btn btn-ghost btn-sm" data-action="restore" data-id="${inst.id}">📥 恢复</button>
        <button class="btn btn-ghost btn-sm" data-action="restart" data-id="${inst.id}">重启</button>
        <button class="btn btn-ghost btn-sm" data-action="stop" data-id="${inst.id}">停止</button>
        <button class="btn btn-ghost btn-sm" data-action="capture-secrets" data-id="${inst.id}">报密钥</button>
        <button class="btn btn-ghost btn-sm" data-action="apply-slots" data-id="${inst.id}">设 Slots</button>
        <button class="btn btn-ghost btn-sm" data-action="recycle" data-id="${inst.id}">回收</button>
        <button class="btn btn-danger btn-sm" data-action="delete" data-id="${inst.id}">删除</button>
    </div>
  </div > `;
}

function bindInstanceActions() {
    $('instances-list').querySelectorAll('[data-action]').forEach(btn => {
        btn.onclick = async () => {
            const { action, id, text } = btn.dataset;
            try {
                if (action === 'copy-delivery') {
                    await copyToClipboard(decodeURIComponent(text));
                    return;
                }
                if (action === 'recycle') {
                    const ok = await confirm('回收实例', '停止容器并解绑客户？数据保留。');
                    if (!ok) return;
                    await api('POST', `/ instances / ${id}/recycle`, { wipe_data: false });
                    showToast('✅ 已回收');
                } else if (action === 'delete') {
                    const ok = await confirm('彻底删除', '将删除容器和数据，无法恢复！');
                    if (!ok) return;
                    await api('DELETE', `/instances/${id}?confirm=true`);
                    showToast('✅ 已删除');
                } else if (action === 'restart') {
                    await api('POST', `/instances/${id}/restart`);
                    showToast('✅ 已重启');
                } else if (action === 'stop') {
                    await api('POST', `/instances/${id}/stop`);
                    showToast('✅ 已停止');
                } else if (action === 'logs') {
                    openLogsModal(id, btn.dataset.name);
                    return;
                } else if (action === 'backup') {
                    // 触发下载
                    window.open(`/api/instances/${id}/backup?token=${encodeURIComponent(state.token)}`, '_blank');
                    return;
                } else if (action === 'restore') {
                    const ok = await confirm('恢复实例', '将上传备份文件覆盖当前数据并重启容器，确认？');
                    if (!ok) return;
                    const input = document.createElement('input');
                    input.type = 'file';
                    input.accept = '.tar.gz,.tgz';
                    input.onchange = async () => {
                        if (!input.files[0]) return;
                        const fd = new FormData();
                        fd.append('file', input.files[0]);
                        try {
                            const res = await fetch(`/api/instances/${id}/restore`, {
                                method: 'POST',
                                headers: { 'X-Admin-Token': state.token },
                                body: fd,
                            });
                            if (!res.ok) throw new Error((await res.json())?.error?.message || 'HTTP ' + res.status);
                            showToast('✅ 恢复成功');
                            await loadInstances();
                        } catch (err) {
                            showToast('❌ ' + err.message, 5000);
                        }
                    };
                    input.click();
                    return;
                } else if (action === 'capture-secrets') {
                    await api('POST', `/instances/${id}/capture-secrets`);
                    showToast('✅ 密钥已重新抓取');
                } else if (action === 'apply-slots') {
                    await api('POST', `/instances/${id}/apply-slots`);
                    showToast('✅ Slots 已应用');
                }
                await loadInstances();
            } catch (err) {
                showToast('❌ ' + err.message, 5000);
            }
        };
    });
}

// === 时间格式化 ===
function formatTime(isoStr) {
    if (!isoStr) return '-';
    const d = new Date(isoStr);
    if (isNaN(d.getTime())) return '-';
    const pad = n => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
function isExpired(isoStr) {
    if (!isoStr) return false;
    return new Date(isoStr) < new Date();
}

// === XSS 防护 ===
function escapeHtml(s) {
    return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// === 日志弹窗 ===
let logsAutoTimer = null;
let logsCurrentId = null;

async function fetchLogs(instanceId) {
    try {
        const data = await api('GET', `/instances/${instanceId}/logs?tail=200`);
        const content = $('logs-content');
        content.textContent = data.logs || '(空日志)';
        // 自动滚到底部
        content.scrollTop = content.scrollHeight;
    } catch (err) {
        $('logs-content').textContent = '❌ 加载失败: ' + err.message;
    }
}

function openLogsModal(instanceId, containerName) {
    logsCurrentId = instanceId;
    $('logs-modal-title').textContent = `容器日志 — ${containerName}`;
    $('logs-content').textContent = '加载中...';
    show($('logs-modal'));
    fetchLogs(instanceId);
    // 启动自动刷新
    if ($('logs-auto-refresh').checked) {
        startLogsAutoRefresh();
    }
}

function closeLogsModal() {
    hide($('logs-modal'));
    stopLogsAutoRefresh();
    logsCurrentId = null;
}

function startLogsAutoRefresh() {
    stopLogsAutoRefresh();
    logsAutoTimer = setInterval(() => {
        if (logsCurrentId) fetchLogs(logsCurrentId);
    }, 5000);
}

function stopLogsAutoRefresh() {
    if (logsAutoTimer) {
        clearInterval(logsAutoTimer);
        logsAutoTimer = null;
    }
}

$('logs-close-btn').addEventListener('click', closeLogsModal);
$('logs-refresh-btn').addEventListener('click', () => {
    if (logsCurrentId) fetchLogs(logsCurrentId);
});
$('logs-auto-refresh').addEventListener('change', e => {
    if (e.target.checked) {
        startLogsAutoRefresh();
    } else {
        stopLogsAutoRefresh();
    }
});
// 点击遮罩关闭
$('logs-modal').addEventListener('click', e => {
    if (e.target === $('logs-modal')) closeLogsModal();
});

// === 从备份恢复（发货页） ===
$('restore-checkout-form').addEventListener('submit', async e => {
    e.preventDefault();
    const fileInput = $('rc-file');
    if (!fileInput.files[0]) {
        showToast('❌ 请选择备份文件');
        return;
    }
    const platformUser = $('rc-platform-user').value.trim();
    if (!platformUser) {
        showToast('❌ 请填写平台用户名');
        return;
    }

    show($('rc-spinner'));
    $('rc-btn-text').textContent = '恢复中…';
    $('rc-btn').disabled = true;

    try {
        const fd = new FormData();
        fd.append('file', fileInput.files[0]);
        fd.append('platform', $('rc-platform').value);
        fd.append('platform_user', platformUser);
        fd.append('duration', $('rc-duration').value);

        const res = await fetch('/api/instances/restore-checkout', {
            method: 'POST',
            headers: { 'X-Admin-Token': state.token },
            body: fd,
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data?.error?.message || 'HTTP ' + res.status);
        showDeliveryResult(data);
        showToast('✅ 从备份恢复成功');
    } catch (err) {
        showToast('❌ ' + err.message, 5000);
    } finally {
        hide($('rc-spinner'));
        $('rc-btn-text').textContent = '上传并恢复';
        $('rc-btn').disabled = false;
    }
});

// === 初始化 ===
(function init() {
    if (state.token) {
        api('GET', '/instances')
            .then(() => showView('main'))
            .catch(() => showView('login'));
    } else {
        showView('login');
    }
})();
