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

function showDeliveryResult(resp) {
    const card = $('delivery-result');
    show(card);
    $('delivery-text').textContent = resp.delivery_text || '';

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
    </div>
    ${inst.last_delivery_text ? `<div class="instance-delivery">${escapeHtml(inst.last_delivery_text)}</div>` : ''}
    <div class="instance-actions">
      <button class="btn btn-ghost btn-sm" data-action="copy-delivery" data-id="${inst.id}" data-text="${encodeURIComponent(inst.last_delivery_text || '')}">
        复制文本
      </button>
      <button class="btn btn-ghost btn-sm" data-action="restart" data-id="${inst.id}">重启</button>
      <button class="btn btn-ghost btn-sm" data-action="stop" data-id="${inst.id}">停止</button>
      <button class="btn btn-ghost btn-sm" data-action="apply-slots" data-id="${inst.id}">设 Slots</button>
      <button class="btn btn-ghost btn-sm" data-action="recycle" data-id="${inst.id}">回收</button>
      <button class="btn btn-danger btn-sm" data-action="delete" data-id="${inst.id}">删除</button>
    </div>
  </div>`;
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
                    await api('POST', `/instances/${id}/recycle`, { wipe_data: false });
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

// === XSS 防护 ===
function escapeHtml(s) {
    return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

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
