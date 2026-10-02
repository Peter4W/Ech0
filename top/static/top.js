/* SPDX-License-Identifier: AGPL-3.0-or-later
 * top.js —— Ech0 动态置顶前端逻辑（注入式，零依赖，不修改 Ech0 本体）。
 * 职责：拦截 fetch 读取置顶标记；给动态卡片加置顶徽标/高亮/分隔；
 *       管理员可直接置顶/取消置顶、在悬浮面板中排序。
 */
(function () {
  'use strict';

  var state = {
    pins: [],          // 有序置顶 ID
    pinSet: {},        // id -> true
    token: (function(){try{return localStorage.getItem('__top_token__')||''}catch(e){return ''}})(),
    isAdmin: false,
    lastIds: [],       // 最近一次 echo/query 返回的有序 ID
    decorated: false,
    userCheckAt: 0
  };

  function isZh() {
    var lang = (document.documentElement.lang || navigator.language || '').toLowerCase();
    return lang.indexOf('zh') === 0;
  }
  var zh = isZh();
  function t(c, e) { return zh ? c : e; }

  // ---------- 工具 ----------
  function captureAuthHeader(headers) {
    if (!headers) return;
    var v = '';
    try {
      if (typeof headers.get === 'function') v = headers.get('Authorization');
    } catch (e) { v = ''; }
    if (!v) v = headers.Authorization || headers.authorization || '';
    if (v && v.toLowerCase().indexOf('bearer ') === 0) {
      state.token = v.slice(7).trim();
      try { localStorage.setItem('__top_token__', state.token); } catch (e) {}
    }
  }

  function apiFetch(path, options) {
    options = options || {};
    options.headers = options.headers || {};
    if (state.token) options.headers.Authorization = 'Bearer ' + state.token;
    return fetch(path, options).then(function (r) { return r.json(); });
  }

  // ---------- fetch / XHR 拦截 ----------
  var origFetch = window.fetch ? window.fetch.bind(window) : null;
  if (origFetch) {
    window.fetch = function (input, init) {
      var url = typeof input === 'string' ? input : (input && input.url) || '';
      captureAuthHeader((init && init.headers) || (input && input.headers));
      return origFetch(input, init).then(function (res) {
        if (url.indexOf('/api/echo/query') >= 0) {
          res.clone().json().then(handleQueryResponse).catch(function () {});
        }
        return res;
      });
    };
  }

  if (window.XMLHttpRequest && XMLHttpRequest.prototype) {
    var origSetHeader = XMLHttpRequest.prototype.setRequestHeader;
    XMLHttpRequest.prototype.setRequestHeader = function (name, value) {
      if (name.toLowerCase() === 'authorization' && typeof value === 'string' &&
          value.toLowerCase().indexOf('bearer ') === 0) {
        state.token = value.slice(7).trim();
        try { localStorage.setItem('__top_token__', state.token); } catch (e) {}
      }
      return origSetHeader.apply(this, arguments);
    };
  }

  function handleQueryResponse(json) {
    var items = json && json.data && json.data.items;
    if (!Array.isArray(items)) return;
    var ids = [];
    items.forEach(function (item) {
      if (item && typeof item.id === 'string') ids.push(item.id);
    });
    state.lastIds = ids;
    scheduleDecorate(true);
  }

  // ---------- 数据加载 ----------
  function loadPins() {
    return fetch('/top/api/pins').then(function (r) { return r.json(); }).then(function (json) {
      var pins = (json && json.data && json.data.pins) || [];
      state.pins = pins;
      state.pinSet = {};
      pins.forEach(function (id) { state.pinSet[id] = true; });
      scheduleDecorate(true);
    }).catch(function () {});
  }

  function checkAdmin() {
    var now = Date.now();
    if (!state.token || now - state.userCheckAt < 3000) return Promise.resolve();
    state.userCheckAt = now;
    return apiFetch('/api/user').then(function (json) {
      state.isAdmin = !!(json && json.code === 1 && json.data && json.data.is_admin);
      scheduleDecorate(true);
    }).catch(function () {});
  }

  // ---------- 卡片装饰 ----------
  function decorate() {
    var roots = Array.prototype.slice.call(document.querySelectorAll('.echo-timeline'));

    // ID → 卡片根元素映射：数量一致时按顺序对齐。
    if (roots.length === state.lastIds.length && roots.length > 0) {
      roots.forEach(function (root, i) {
        root.setAttribute('data-top-id', state.lastIds[i]);
      });
    }

    var firstNormalMarked = false;
    var listStartedWithPins = state.lastIds.length > 0 && !!state.pinSet[state.lastIds[0]];

    roots.forEach(function (root) {
      var id = root.getAttribute('data-top-id');
      if (!id) return;
      var pinned = !!state.pinSet[id];

      if (pinned) {
        root.setAttribute('data-top-pinned', '1');
        root.removeAttribute('data-top-after-pin');
        ensureBadge(root);
      } else {
        root.removeAttribute('data-top-pinned');
        removeBadge(root);
        // 置顶区结束后的第一条普通动态：插入"最新动态"分隔（仅首页顺序成立时）。
        if (listStartedWithPins && !firstNormalMarked) {
          root.setAttribute('data-top-after-pin', '1');
          firstNormalMarked = true;
        } else {
          root.removeAttribute('data-top-after-pin');
        }
      }

      if (state.isAdmin) ensureAdminButton(root, id, pinned);
      else removeAdminButton(root);
    });

    ensureManager();
  }

  function ensureBadge(root) {
    if (root.querySelector(':scope > .top-pin-badge')) return;
    var span = document.createElement('span');
    span.className = 'top-pin-badge';
    span.textContent = '📌 ' + t('置顶', 'Pinned');
    root.insertBefore(span, root.firstChild);
  }

  function removeBadge(root) {
    var badge = root.querySelector(':scope > .top-pin-badge');
    if (badge) badge.parentNode.removeChild(badge);
  }

  function ensureAdminButton(root, id, pinned) {
    var host = root.querySelector('.echo-header-sticky > div.relative');
    if (!host) return;
    var btn = host.querySelector(':scope > .top-pin-btn');
    if (!btn) {
      btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'top-pin-btn';
      host.insertBefore(btn, host.firstChild);
    }
    var wantText = pinned ? '✕ ' + t('取消置顶', 'Unpin') : '📌 ' + t('置顶', 'Pin');
    if (btn.textContent !== wantText) btn.textContent = wantText;
    if (btn.dataset.pinned !== (pinned ? '1' : '0')) btn.dataset.pinned = pinned ? '1' : '0';
    btn.onclick = function (ev) {
      ev.stopPropagation();
      btn.disabled = true;
      var job = pinned
        ? apiFetch('/top/api/pins/' + encodeURIComponent(id), { method: 'DELETE' })
        : apiFetch('/top/api/pins', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: id })
          });
      job.then(function (json) {
        if (json && json.code === 1) {
          window.location.reload();
        } else {
          btn.disabled = false;
          alert((json && json.msg) || t('操作失败', 'operation failed'));
        }
      }).catch(function () { btn.disabled = false; });
    };
  }

  function removeAdminButton(root) {
    var btn = root.querySelector('.echo-header-sticky .top-pin-btn');
    if (btn) btn.parentNode.removeChild(btn);
  }

  // ---------- 悬浮置顶管理面板 ----------
  function ensureManager() {
    if (!state.isAdmin) {
      var old = document.getElementById('top-manager');
      if (old) old.parentNode.removeChild(old);
      return;
    }
    if (document.getElementById('top-manager')) return;

    var box = document.createElement('div');
    box.id = 'top-manager';
    box.innerHTML =
      '<button type="button" class="top-manager-fab" title="' + t('置顶管理', 'Pin manager') + '">📌</button>' +
      '<div class="top-manager-panel" hidden>' +
      '  <div class="top-manager-head"><strong>' + t('置顶管理', 'Pinned posts') + '</strong>' +
      '    <button type="button" class="top-manager-close">✕</button></div>' +
      '  <ul class="top-manager-list"></ul>' +
      '</div>';
    document.body.appendChild(box);

    var fab = box.querySelector('.top-manager-fab');
    var panel = box.querySelector('.top-manager-panel');
    fab.addEventListener('click', function () {
      panel.hidden = !panel.hidden;
      if (!panel.hidden) renderManagerList();
    });
    box.querySelector('.top-manager-close').addEventListener('click', function () {
      panel.hidden = true;
    });
  }

  function renderManagerList() {
    var ul = document.querySelector('#top-manager .top-manager-list');
    if (!ul) return;
    ul.innerHTML = '';
    if (state.pins.length === 0) {
      var li = document.createElement('li');
      li.className = 'top-manager-empty';
      li.textContent = t('暂无置顶动态', 'No pinned posts yet');
      ul.appendChild(li);
      return
    }
    state.pins.forEach(function (id, i) {
      var li = document.createElement('li');
      li.className = 'top-manager-item';
      var name = document.createElement('span');
      name.className = 'top-manager-name';
      name.textContent = (i + 1) + '. ' + id;
      name.title = id;
      li.appendChild(name);

      var mk = function (sym, title, disabled, fn) {
        var b = document.createElement('button');
        b.type = 'button';
        b.textContent = sym;
        b.title = title;
        b.disabled = disabled;
        b.addEventListener('click', fn);
        li.appendChild(b);
        return b;
      };
      mk('↑', t('上移', 'up'), i === 0, function () { movePin(i, i - 1); });
      mk('↓', t('下移', 'down'), i === state.pins.length - 1, function () { movePin(i, i + 1); });
      mk('✕', t('取消置顶', 'unpin'), false, function () {
        apiFetch('/top/api/pins/' + encodeURIComponent(id), { method: 'DELETE' })
          .then(afterMutation);
      });
      ul.appendChild(li);
    });
  }

  function movePin(from, to) {
    var ids = state.pins.slice();
    var tmp = ids[from];
    ids[from] = ids[to];
    ids[to] = tmp;
    apiFetch('/top/api/pins/reorder', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ids: ids })
    }).then(afterMutation);
  }

  function afterMutation(json) {
    if (json && json.code === 1) {
      loadPins().then(function () { window.location.reload(); });
    } else {
      alert((json && json.msg) || t('操作失败', 'operation failed'));
    }
  }

  // ---------- 调度 ----------
  var decorateTimer = 0;
  var decorateLaterTimer = 0;
  function scheduleDecorate(immediate) {
    if (decorateTimer) clearTimeout(decorateTimer);
    decorateTimer = setTimeout(function () {
      decorate();
      checkAdmin();
    }, immediate ? 60 : 120);
    // TransitionGroup 入场动画结束后再补一次。
    if (decorateLaterTimer) clearTimeout(decorateLaterTimer);
    decorateLaterTimer = setTimeout(decorate, 800);
  }

  var observer = new MutationObserver(function (mutations) {
    for (var i = 0; i < mutations.length; i++) {
      var target = mutations[i].target;
      // 面板内部变动（展开/列表渲染）不触发全局装饰。
      if (target && target.closest && target.closest('#top-manager')) continue;
      scheduleDecorate(false);
      return;
    }
  });

  function start() {
    loadPins();
    observer.observe(document.body, { childList: true, subtree: true });
    scheduleDecorate(true);
    window.addEventListener('focus', function () { loadPins(); checkAdmin(); });
    // 令牌通常在首屏请求中出现，延迟补查管理员身份。
    setTimeout(checkAdmin, 500);
    setTimeout(checkAdmin, 2000);
    // 监听 /api/user 返回非管理员时清除令牌（处理登出场景）。
    setInterval(function () {
      if (state.token && state.userCheckAt > 0 && !state.isAdmin) {
        state.token = '';
        try { localStorage.removeItem('__top_token__'); } catch (e) {}
      }
    }, 5000);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', start);
  } else {
    start();
  }
})();
