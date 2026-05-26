// projectManager web 客户端
// 严格按照 docs/http_api.md 对接，字段命名 snake_case。

(() => {
  // ---------- 身份信息 ----------
  const Auth = {
    get() {
      return {
        id: localStorage.getItem('user_id') || '',
        name: localStorage.getItem('user_name') || '',
        role: localStorage.getItem('user_role') || 'applicant',
      };
    },
    set(u) {
      localStorage.setItem('user_id', u.id || '');
      localStorage.setItem('user_name', u.name || '');
      localStorage.setItem('user_role', u.role || 'applicant');
    },
  };

  // ---------- HTTP 封装 ----------
  async function api(path, options = {}) {
    const u = Auth.get();
    if (!u.id) throw new Error('请先在右上角填写用户ID与角色');
    const headers = Object.assign(
      {
        'Content-Type': 'application/json',
        'X-User-Id': u.id,
        'X-User-Role': u.role,
        'X-User-Name': u.name,
      },
      options.headers || {}
    );
    const res = await fetch(path, { ...options, headers });
    let data = null;
    try { data = await res.json(); } catch (_) { /* ignore */ }
    if (!data) {
      throw { code: -1, message: 'NETWORK_ERROR', http: res.status };
    }
    if (data.code !== 0) throw data;
    return data.data;
  }

  // ---------- 工具 ----------
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
  const fmt = (v) => {
    if (v === null || v === undefined) return '';
    if (typeof v === 'object') return JSON.stringify(v);
    return String(v);
  };
  const fmtTime = (t) => {
    if (!t) return '';
    try { return new Date(t).toLocaleString(); } catch (_) { return String(t); }
  };
  const showMsg = (el, text, ok = true) => {
    if (!el) return;
    el.textContent = text;
    el.className = 'msg ' + (ok ? 'ok' : 'err');
  };

  // ---------- 用户身份栏 ----------
  function bindUserBar() {
    const u = Auth.get();
    $('#user_id').value = u.id;
    $('#user_name').value = u.name;
    $('#user_role').value = u.role || 'applicant';
    $('#save_user_btn').addEventListener('click', () => {
      Auth.set({
        id: $('#user_id').value.trim(),
        name: $('#user_name').value.trim(),
        role: $('#user_role').value,
      });
      alert('身份已保存');
    });
  }

  // ---------- 路由 / 视图切换 ----------
  const Views = {
    list: renderList,
    create: renderCreate,
    todos: renderTodos,
    detail: renderDetail,
  };
  function go(view, params) {
    $$('.tabs .tab').forEach((t) => t.classList.toggle('active', t.dataset.view === view));
    const fn = Views[view];
    if (!fn) return;
    fn(params || {});
  }
  function bindTabs() {
    $$('.tabs .tab').forEach((t) => t.addEventListener('click', () => go(t.dataset.view)));
  }

  // ---------- 列表 ----------
  async function renderList() {
    const tpl = $('#tpl-list').content.cloneNode(true);
    const main = $('#main');
    main.innerHTML = '';
    main.appendChild(tpl);

    const tbody = $('#list_tbody');
    const totalSpan = $('#list_total');

    async function load() {
      tbody.innerHTML = '<tr><td colspan="8">加载中…</td></tr>';
      try {
        const params = new URLSearchParams();
        const kw = $('#f_keyword').value.trim();
        const st = $('#f_status').value;
        if (kw) params.set('keyword', kw);
        if (st) params.set('status', st);
        params.set('size', '50');
        const data = await api('/api/v1/projects?' + params.toString());
        renderRows(data);
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="8" class="msg err">${e.message || e.code}</td></tr>`;
      }
    }

    function renderRows(data) {
      tbody.innerHTML = '';
      if (!data.items.length) {
        tbody.innerHTML = '<tr><td colspan="8">暂无数据</td></tr>';
        totalSpan.textContent = '共 0 条';
        return;
      }
      data.items.forEach((p) => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
          <td>${fmt(p.project_code)}</td>
          <td><span class="action-link">${fmt(p.project_name)}</span></td>
          <td>${fmt(p.owner_name)}</td>
          <td><span class="badge ${p.status}">${p.status}</span></td>
          <td>R${p.current_revision}</td>
          <td>${fmt(p.applicant_name)}</td>
          <td>${fmtTime(p.updated_at)}</td>
          <td><span class="action-link" data-id="${p.id}">查看</span></td>
        `;
        tr.querySelector('.action-link').addEventListener('click', () => go('detail', { id: p.id }));
        tr.querySelector('[data-id]').addEventListener('click', () => go('detail', { id: p.id }));
        tbody.appendChild(tr);
      });
      totalSpan.textContent = `共 ${data.total} 条`;
    }

    $('#btn_search').addEventListener('click', load);
    load();
  }

  // ---------- 立项 ----------
  async function renderCreate() {
    const tpl = $('#tpl-create').content.cloneNode(true);
    $('#main').innerHTML = '';
    $('#main').appendChild(tpl);

    $('#btn_create').addEventListener('click', async () => {
      const body = {
        project_name: $('#c_project_name').value.trim(),
        owner: {
          name: $('#c_owner_name').value.trim(),
          owner_type: $('#c_owner_type').value,
          contact_name: $('#c_contact_name').value.trim(),
          contact_phone: $('#c_contact_phone').value.trim(),
          contact_email: $('#c_contact_email').value.trim(),
          address: $('#c_address').value.trim(),
        },
      };
      if (!body.project_name) {
        showMsg($('#create_msg'), '项目名称必填', false);
        return;
      }
      if (!body.owner.name) {
        showMsg($('#create_msg'), '业主单位名称必填', false);
        return;
      }
      try {
        const p = await api('/api/v1/projects', {
          method: 'POST',
          body: JSON.stringify(body),
        });
        showMsg($('#create_msg'), `立项成功，编号 ${p.project_code}${p.duplicate_active ? '（注意：存在同名同业主未归档项目）' : ''}`, true);
        setTimeout(() => go('detail', { id: p.id }), 600);
      } catch (e) {
        showMsg($('#create_msg'), '立项失败：' + (e.message || JSON.stringify(e)), false);
      }
    });
  }

  // ---------- 待办 ----------
  async function renderTodos() {
    const tpl = $('#tpl-todos').content.cloneNode(true);
    $('#main').innerHTML = '';
    $('#main').appendChild(tpl);
    const tbody = $('#todo_tbody');
    tbody.innerHTML = '<tr><td colspan="7">加载中…</td></tr>';
    try {
      const data = await api('/api/v1/me/todos');
      tbody.innerHTML = '';
      if (!data.items.length) {
        tbody.innerHTML = '<tr><td colspan="7">暂无待办</td></tr>';
        return;
      }
      data.items.forEach((p) => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
          <td>${fmt(p.project_code)}</td>
          <td>${fmt(p.project_name)}</td>
          <td>${fmt(p.owner_name)}</td>
          <td><span class="badge ${p.status}">${p.status}</span></td>
          <td>R${p.current_revision}</td>
          <td>${fmtTime(p.updated_at)}</td>
          <td><span class="action-link">处理</span></td>
        `;
        tr.querySelector('.action-link').addEventListener('click', () => go('detail', { id: p.id }));
        tbody.appendChild(tr);
      });
    } catch (e) {
      tbody.innerHTML = `<tr><td colspan="7" class="msg err">${e.message || e.code}</td></tr>`;
    }
  }

  // ---------- 详情 ----------
  async function renderDetail({ id }) {
    if (!id) return go('list');
    const tpl = $('#tpl-detail').content.cloneNode(true);
    $('#main').innerHTML = '';
    $('#main').appendChild(tpl);

    let formData = null;     // { project, template, values }
    let editValues = {};     // 当前编辑态值

    $('#btn_back').addEventListener('click', () => go('list'));

    // 子 tab 切换
    $$('.sub-tab').forEach((t) => t.addEventListener('click', () => {
      $$('.sub-tab').forEach((x) => x.classList.toggle('active', x === t));
      $$('.pane').forEach((p) => p.classList.add('hidden'));
      const pane = $('#pane_' + t.dataset.pane);
      pane.classList.remove('hidden');
      if (t.dataset.pane === 'logs') loadLogs();
      if (t.dataset.pane === 'approvals') loadApprovals();
    }));

    async function loadForm() {
      try {
        formData = await api(`/api/v1/projects/${id}/form`);
        const { project, template, values } = formData;
        editValues = { ...values };
        $('#d_title').textContent = `${project.project_name}（${project.project_code}）`;
        const status = project.status;
        const statusEl = $('#d_status');
        statusEl.textContent = status;
        statusEl.className = 'badge ' + status;
        $('#d_rev').textContent = project.current_revision;
        $('#d_applicant').textContent = project.applicant_name || project.applicant_id;

        renderFormPane(template, values, project);
        toggleActions(project);
      } catch (e) {
        showMsg($('#detail_msg'), '加载失败：' + (e.message || JSON.stringify(e)), false);
      }
    }

    function renderFormPane(template, values, project) {
      const pane = $('#pane_form');
      pane.innerHTML = '';
      const editable = project.status === 'FORM_EDITING';
      const role = Auth.get().role;
      template.sections.forEach((sec) => {
        const block = document.createElement('div');
        block.className = 'section-block';
        block.innerHTML = `<div class="section-title">${sec.section_label}</div>`;
        const fl = document.createElement('div');
        fl.className = 'field-list';
        sec.fields.forEach((f) => {
          const item = document.createElement('div');
          const useFull = ['textarea', 'table'].includes(f.type);
          item.className = 'field-item' + (useFull ? ' full' : '');
          item.dataset.field = f.field_key;
          const labelHtml = `<div class="label">${f.label}${f.required ? '<span class="req">*</span>' : ''}<span style="color:#94a3b8;margin-left:6px;">${f.field_key}</span></div>`;
          const cur = values[f.field_key];
          const canEdit = editable && (role === 'admin' || (f.editable_roles || []).includes(role));
          item.innerHTML = labelHtml + buildEditor(f, cur, canEdit);
          fl.appendChild(item);
          const editor = item.querySelector('[data-input]');
          if (editor && canEdit) {
            editor.addEventListener('input', () => {
              const v = readEditor(editor, f.type);
              editValues[f.field_key] = v;
              const changed = !deepEqual(v, values[f.field_key]);
              item.classList.toggle('changed', changed);
            });
          }
        });
        block.appendChild(fl);
        pane.appendChild(block);
      });
      // 保存按钮
      if (project.status === 'FORM_EDITING') {
        const bar = document.createElement('div');
        bar.className = 'actions';
        bar.innerHTML = `<button id="btn_save_form" class="primary">保存修改</button>
                        <input id="save_remark" placeholder="本次修改说明（可选）" style="flex:1;padding:6px 10px;border:1px solid #cbd5e1;border-radius:4px;" />`;
        pane.appendChild(bar);
        $('#btn_save_form').addEventListener('click', saveForm);
      }
    }

    function buildEditor(f, val, canEdit) {
      const dis = canEdit ? '' : 'disabled';
      const v = val === null || val === undefined ? '' : val;
      switch (f.type) {
        case 'textarea':
          return `<textarea data-input ${dis} placeholder="${f.placeholder || ''}">${escapeHtml(fmt(v))}</textarea>`;
        case 'number':
        case 'money':
          return `<input data-input type="number" ${dis} value="${escapeAttr(fmt(v))}" />`;
        case 'date':
          return `<input data-input type="date" ${dis} value="${escapeAttr(fmt(v))}" />`;
        case 'select':
          return `<select data-input ${dis}>
            <option value="">请选择</option>
            ${(f.options || []).map(o => `<option value="${escapeAttr(o)}" ${o == v ? 'selected' : ''}>${escapeHtml(o)}</option>`).join('')}
          </select>`;
        case 'multiselect':
          return `<input data-input ${dis} value="${escapeAttr(Array.isArray(v) ? v.join(',') : fmt(v))}" placeholder="多个值用逗号分隔" />`;
        default:
          return `<input data-input type="text" ${dis} value="${escapeAttr(fmt(v))}" placeholder="${f.placeholder || ''}" />`;
      }
    }

    function readEditor(el, type) {
      const raw = el.value;
      if (raw === '' || raw === undefined) return null;
      if (type === 'number' || type === 'money') {
        const n = Number(raw);
        return isNaN(n) ? raw : n;
      }
      if (type === 'multiselect') {
        return raw.split(',').map((s) => s.trim()).filter(Boolean);
      }
      return raw;
    }

    function deepEqual(a, b) {
      if (a === b) return true;
      if (a == null || b == null) return a == b;
      if (typeof a !== typeof b) return false;
      if (typeof a === 'object') return JSON.stringify(a) === JSON.stringify(b);
      return false;
    }

    function escapeHtml(s) {
      return String(s).replace(/[&<>]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c]));
    }
    function escapeAttr(s) {
      return String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
    }

    async function saveForm() {
      const remark = $('#save_remark').value.trim();
      const original = formData.values;
      const changes = [];
      Object.keys(editValues).forEach((k) => {
        if (!deepEqual(editValues[k], original[k])) {
          changes.push({ field_key: k, new_value: editValues[k], remark });
        }
      });
      if (!changes.length) {
        showMsg($('#detail_msg'), '没有任何字段变化', false);
        return;
      }
      try {
        const res = await api(`/api/v1/projects/${id}/form`, {
          method: 'PATCH',
          body: JSON.stringify({ changes }),
        });
        showMsg($('#detail_msg'), `保存成功：revision=${res.revision}, 写入 ${res.applied_count} 条记录`, true);
        await loadForm();
      } catch (e) {
        showMsg($('#detail_msg'), '保存失败：' + (e.message || JSON.stringify(e)), false);
      }
    }

    function toggleActions(project) {
      const role = Auth.get().role;
      const status = project.status;
      const map = {
        btn_submit_final: status === 'FORM_EDITING' && (role === 'applicant' || role === 'admin'),
        btn_review_approve: status === 'PENDING_REVIEW' && role === 'reviewer',
        btn_review_reject: status === 'PENDING_REVIEW' && role === 'reviewer',
        btn_final_approve: status === 'PENDING_APPROVE' && role === 'approver',
        btn_final_reject: status === 'PENDING_APPROVE' && role === 'approver',
      };
      Object.entries(map).forEach(([id, en]) => {
        const el = document.getElementById(id);
        if (!el) return;
        el.disabled = !en;
      });
    }

    $('#btn_submit_final').addEventListener('click', async () => {
      try {
        const res = await api(`/api/v1/projects/${id}/submit`, { method: 'POST' });
        showMsg($('#detail_msg'), `提交定稿成功，进入 ${res.status}`, true);
        loadForm();
      } catch (e) {
        if (e && e.code === 1006) {
          showMsg($('#detail_msg'), '校验失败，缺少字段：' + (e.data && e.data.missing_fields ? e.data.missing_fields.join(', ') : ''), false);
        } else {
          showMsg($('#detail_msg'), '提交失败：' + (e.message || JSON.stringify(e)), false);
        }
      }
    });

    async function decide(level, action) {
      let comment = '';
      if (action === 'reject') {
        comment = prompt('请输入驳回意见：') || '';
        if (!comment.trim()) {
          showMsg($('#detail_msg'), '驳回意见必填', false);
          return;
        }
      }
      try {
        const path = level === 1 ? 'review' : 'final';
        const res = await api(`/api/v1/projects/${id}/approvals/${path}`, {
          method: 'POST',
          body: JSON.stringify({ action, comment }),
        });
        showMsg($('#detail_msg'), `${action === 'approve' ? '通过' : '驳回'}成功，状态 ${res.status}`, true);
        loadForm();
      } catch (e) {
        showMsg($('#detail_msg'), '操作失败：' + (e.message || JSON.stringify(e)), false);
      }
    }
    $('#btn_review_approve').addEventListener('click', () => decide(1, 'approve'));
    $('#btn_review_reject').addEventListener('click', () => decide(1, 'reject'));
    $('#btn_final_approve').addEventListener('click', () => decide(2, 'approve'));
    $('#btn_final_reject').addEventListener('click', () => decide(2, 'reject'));

    // ---- 修改记录 ----
    async function loadLogs() {
      const tbody = $('#log_tbody');
      tbody.innerHTML = '<tr><td colspan="9">加载中…</td></tr>';
      try {
        const params = new URLSearchParams();
        const fk = $('#lf_field_key').value.trim();
        const op = $('#lf_operator_id').value.trim();
        if (fk) params.set('field_key', fk);
        if (op) params.set('operator_id', op);
        params.set('size', '100');
        const data = await api(`/api/v1/projects/${id}/changelogs?${params.toString()}`);
        tbody.innerHTML = '';
        if (!data.items.length) {
          tbody.innerHTML = '<tr><td colspan="9">暂无记录</td></tr>';
          return;
        }
        data.items.forEach((l) => {
          const tr = document.createElement('tr');
          tr.innerHTML = `
            <td>R${l.revision}</td>
            <td><div>${fmt(l.field_label)}</div><div style="color:#94a3b8;font-size:12px;">${fmt(l.field_key)}</div></td>
            <td>${escapeHtml(fmt(l.old_value))}</td>
            <td>${escapeHtml(fmt(l.new_value))}</td>
            <td>${fmt(l.operator_name) || fmt(l.operator_id)}</td>
            <td>${fmt(l.operator_role)}</td>
            <td>${fmtTime(l.operated_at)}</td>
            <td>${fmt(l.phase)}</td>
            <td>${escapeHtml(fmt(l.remark))}</td>
          `;
          tbody.appendChild(tr);
        });
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="9" class="msg err">${e.message || e.code}</td></tr>`;
      }
    }
    $('#btn_log_search').addEventListener('click', loadLogs);
    $('#btn_diff').addEventListener('click', async () => {
      const from = parseInt($('#diff_from').value || '0', 10);
      const to = parseInt($('#diff_to').value || '0', 10);
      if (!to) {
        showMsg($('#detail_msg'), '请输入 to revision', false);
        return;
      }
      try {
        const data = await api(`/api/v1/projects/${id}/diff?from=${from}&to=${to}`);
        const tbody = $('#log_tbody');
        tbody.innerHTML = '';
        if (!data.items.length) {
          tbody.innerHTML = '<tr><td colspan="9">两个版本之间无差异</td></tr>';
          return;
        }
        data.items.forEach((l) => {
          const tr = document.createElement('tr');
          tr.className = 'diff-row';
          tr.innerHTML = `
            <td>R${from}→R${to}</td>
            <td><div>${fmt(l.field_label)}</div><div style="color:#94a3b8;font-size:12px;">${fmt(l.field_key)}</div></td>
            <td>${escapeHtml(fmt(l.old_value))}</td>
            <td>${escapeHtml(fmt(l.new_value))}</td>
            <td colspan="5">折叠后：起点旧值 → 终点新值</td>
          `;
          tbody.appendChild(tr);
        });
      } catch (e) {
        showMsg($('#detail_msg'), '差异查询失败：' + (e.message || JSON.stringify(e)), false);
      }
    });

    // ---- 审批事件 ----
    async function loadApprovals() {
      const tbody = $('#approval_tbody');
      tbody.innerHTML = '<tr><td colspan="6">加载中…</td></tr>';
      try {
        const data = await api(`/api/v1/projects/${id}/approvals`);
        tbody.innerHTML = '';
        if (!data.items.length) {
          tbody.innerHTML = '<tr><td colspan="6">暂无</td></tr>';
          return;
        }
        data.items.forEach((e) => {
          const tr = document.createElement('tr');
          tr.innerHTML = `
            <td>L${e.level}</td>
            <td>${fmt(e.action)}</td>
            <td>${fmt(e.operator_name) || fmt(e.operator_id)}</td>
            <td>${fmt(e.operator_role)}</td>
            <td>${escapeHtml(fmt(e.comment))}</td>
            <td>${fmtTime(e.created_at)}</td>
          `;
          tbody.appendChild(tr);
        });
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="6" class="msg err">${e.message || e.code}</td></tr>`;
      }
    }

    function escapeHtml(s) {
      return String(s).replace(/[&<>]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c]));
    }

    loadForm();
  }

  // ---------- 启动 ----------
  document.addEventListener('DOMContentLoaded', () => {
    bindUserBar();
    bindTabs();
    if (!Auth.get().id) {
      // 默认填充一个演示身份
      Auth.set({ id: 'u-001', name: '张三', role: 'applicant' });
      $('#user_id').value = 'u-001';
      $('#user_name').value = '张三';
    }
    go('list');
  });
})();
