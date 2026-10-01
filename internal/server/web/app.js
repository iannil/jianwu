/* jianwu web UI (vanilla JS, no build step) */
"use strict";

// ---------- API client ----------

async function api(path, opts = {}) {
  const init = { headers: {} };
  if (opts.body !== undefined) {
    init.method = opts.method || "POST";
    init.headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(opts.body);
  } else if (opts.method) {
    init.method = opts.method;
  }
  const resp = await fetch("/api/v1" + path, init);
  let data = {};
  try { data = await resp.json(); } catch { /* empty body */ }
  if (!resp.ok) {
    const msg = data && data.error ? data.error : `HTTP ${resp.status}`;
    const err = new Error(msg);
    err.status = resp.status;
    throw err;
  }
  return data;
}

// ---------- helpers ----------

const $ = (sel, el = document) => el.querySelector(sel);

function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, c => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[c]));
}

function toast(msg, kind = "") {
  const el = document.createElement("div");
  el.className = "toast " + kind;
  // 黑白灰配色下用前缀符号区分成败
  el.textContent = (kind === "err" ? "✕ " : kind === "ok" ? "✓ " : "") + msg;
  $("#toasts").appendChild(el);
  setTimeout(() => el.remove(), kind === "err" ? 6000 : 3000);
}

// 内置对话框，替代原生 confirm/prompt。input=false 时 close(true)；有输入框时 close(输入值)。
const Modal = {
  _resolve: null,

  open({ title, text = "", input = false, value = "", placeholder = "", okText = "确定", danger = false }) {
    return new Promise(resolve => {
      this._resolve = resolve;
      $("#modal-title").textContent = title;
      const textEl = $("#modal-text");
      textEl.textContent = text;
      textEl.classList.toggle("hidden", !text);
      $("#modal-input-row").classList.toggle("hidden", !input);
      const inp = $("#modal-input");
      inp.value = value;
      inp.placeholder = placeholder;
      $("#modal-ok").textContent = okText;
      $("#modal-ok").className = danger ? "btn danger-solid" : "btn primary";
      $("#modal").classList.remove("hidden");
      setTimeout(() => (input ? inp : $("#modal-ok")).focus(), 30);
    });
  },

  close(result) {
    if (!this._resolve) return;
    $("#modal").classList.add("hidden");
    const resolve = this._resolve;
    this._resolve = null;
    resolve(result);
  },

  submit() {
    const hasInput = !$("#modal-input-row").classList.contains("hidden");
    this.close(hasInput ? $("#modal-input").value.trim() : true);
  },
};

// 视图加载骨架。
const SKELETON_VIEW = `
  <div class="skeleton" style="width:220px;height:30px;margin-bottom:12px"></div>
  <div class="skeleton" style="width:320px;height:15px;margin-bottom:32px"></div>
  <div class="skeleton" style="height:110px;margin-bottom:14px"></div>
  <div class="skeleton" style="height:260px"></div>`;

function emptyState(glyph, title, text, actionHtml = "") {
  return `<div class="empty"><div class="glyph" aria-hidden="true">${glyph}</div><h3>${title}</h3><p>${text}</p>${actionHtml}</div>`;
}

const STATUS_ZH = {
  scaffolded: "待展开", expanded: "已展开", reviewed: "已审阅",
  final: "已定稿", failed: "失败", draft: "草稿",
};
const STATUS_COLOR = {
  scaffolded: "#d2d2d2", expanded: "#ababab", reviewed: "#828282",
  final: "#1c1c1c", failed: "#545454",
};
function statusBadge(s) {
  return `<span class="badge s-${esc(s)}">${STATUS_ZH[s] || esc(s)}</span>`;
}
function fmtTokens(u) {
  if (!u || !u.call_count) return "—";
  return `${u.total_tokens} tokens（${u.call_count} 次调用）`;
}

// Minimal markdown renderer: headings, bold/italic/code, footnotes, lists, paragraphs.
function renderMD(src) {
  const lines = String(src || "").split("\n");
  let html = "", inList = false, inCode = false;
  const inline = (s) => esc(s)
    .replace(/\[\^([^\]]+)\]/g, '<sup class="fn">[$1]</sup>')
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
    .replace(/\*([^*]+)\*/g, "<em>$1</em>")
    .replace(/`([^`]+)`/g, "<code>$1</code>")
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
  for (const raw of lines) {
    if (raw.startsWith("```")) { inCode = !inCode; html += inCode ? "<pre>" : "</pre>"; continue; }
    if (inCode) { html += esc(raw) + "\n"; continue; }
    const line = raw.trimEnd();
    const m = line.match(/^(#{1,4})\s+(.*)$/);
    if (m) {
      if (inList) { html += "</ul>"; inList = false; }
      html += `<h${m[1].length + 1}>${inline(m[2])}</h${m[1].length + 1}>`;
    } else if (/^\s*[-*]\s+/.test(line)) {
      if (!inList) { html += "<ul>"; inList = true; }
      html += `<li>${inline(line.replace(/^\s*[-*]\s+/, ""))}</li>`;
    } else if (line.trim() === "") {
      if (inList) { html += "</ul>"; inList = false; }
    } else {
      if (inList) { html += "</ul>"; inList = false; }
      html += `<p>${inline(line)}</p>`;
    }
  }
  if (inList) html += "</ul>";
  if (inCode) html += "</pre>";
  return html;
}

// ---------- job polling ----------

const Jobs = {
  known: new Map(), // id -> status
  timer: null,

  start() { this.timer = setInterval(() => this.refresh(), 1500); },

  async refresh() {
    try {
      const { jobs } = await api("/jobs");
      const running = jobs.filter(j => j.status === "running").length;
      const badge = $("#jobs-badge");
      badge.textContent = running > 0 ? `任务 (${running}●)` : "任务";
      for (const j of jobs) {
        const prev = this.known.get(j.id);
        if (prev && prev !== "succeeded" && j.status === "succeeded") {
          toast(`任务完成：${this.kindZh(j.kind)}${j.slug ? " · " + j.slug : ""}`, "ok");
          if (App.onJobDone) App.onJobDone(j);
        } else if (prev && prev !== "failed" && j.status === "failed") {
          toast(`任务失败：${j.error || "未知错误"}`, "err");
          if (App.onJobDone) App.onJobDone(j);
        }
        this.known.set(j.id, j.status);
      }
      if (!$("#jobs-drawer").classList.contains("hidden")) this.renderList(jobs);
    } catch { /* server unreachable; ignore */ }
  },

  kindZh(kind) {
    return { "new-book": "生成图书", expand: "展开章节", "expand-all": "批量展开",
      factcheck: "事实核查", revise: "修订", export: "导出", "corpus-reindex": "语料重建索引",
      "corpus-collect": "语料采集" }[kind] || kind;
  },

  async renderList() {
    const { jobs } = await api("/jobs");
    const el = $("#jobs-list");
    if (!jobs.length) { el.innerHTML = `<div class="empty">暂无任务</div>`; return; }
    el.innerHTML = jobs.map(j => `
      <div class="job-item">
        <div class="job-top">
          <b>${this.kindZh(j.kind)}</b>
          ${j.slug ? `<span class="mono muted">${esc(j.slug)}</span>` : ""}
          ${j.status === "running"
            ? `<button class="btn sm danger" onclick="App.cancelJob('${j.id}')">取消</button>`
            : `<span class="badge ${j.status === "succeeded" ? "v-ok" : "v-bad"}">${j.status === "succeeded" ? "成功" : "失败"}</span>`}
        </div>
        ${j.status === "running" ? `<div class="job-bar"><i style="width:${j.progress}%"></i></div>` : ""}
        <div class="muted" style="font-size:12px">${esc(j.message || "")}${j.error ? " · " + esc(j.error) : ""}</div>
        ${j.log ? `<div class="job-log">${esc(j.log)}</div>` : ""}
      </div>`).join("");
  },
};

// ---------- app ----------

const App = {
  ws: null,
  onJobDone: null,

  async boot() {
    window.addEventListener("hashchange", () => this.route());
    Jobs.start();
    try {
      this.ws = await api("/workspace");
      $("#brand-version").textContent = "jianwu " + (this.ws.version || "");
    } catch (e) {
      toast("无法连接服务端：" + e.message, "err");
    }
    this.route();
  },

  setNav(name) {
    document.querySelectorAll("#main-nav a").forEach(a =>
      a.classList.toggle("active", a.dataset.nav === name));
  },

  async route() {
    const hash = location.hash || "#/";
    const parts = hash.slice(2).split("/").filter(Boolean);
    this.onJobDone = null;
    try {
      if (parts.length === 0) return this.viewDashboard();
      if (parts[0] === "book" && parts[1]) return this.viewBook(parts[1]);
      if (parts[0] === "books") return this.viewBooks();
      if (parts[0] === "new") return this.viewNewWizard();
      if (parts[0] === "corpus") return this.viewCorpus();
      if (parts[0] === "config") return this.viewConfig();
      return this.viewDashboard();
    } catch (e) {
      $("#view").innerHTML = `<div class="card"><b>加载失败</b><div class="muted">${esc(e.message)}</div></div>`;
    }
  },

  async refreshWs() { this.ws = await api("/workspace"); },

  // ----- views -----

  SOURCE_ZH: {
    flag: "命令行参数 --dir", env: "环境变量 JIANWU_WORKSPACE",
    config: "全局配置文件", cwd: "启动目录", web: "网页设置", startup: "启动参数",
  },

  workspaceForm(highlight) {
    const src = this.SOURCE_ZH[this.ws.source] || this.ws.source || "未知";
    return `
      <div class="card" ${highlight ? 'style="max-width:640px;margin:40px auto"' : ""}>
        <div class="kv">
          <span class="k">当前工作区</span><span class="mono">${esc(this.ws.root)}</span>
          <span class="k">配置来源</span><span>${esc(src)}</span>
          ${this.ws.initialized ? `<span class="k">状态</span><span><span class="badge v-ok">已初始化</span></span>`
                                : `<span class="k">状态</span><span><span class="badge s-failed">未初始化</span></span>`}
        </div>
        ${highlight ? `<p class="muted" style="font-size:13px;margin:12px 0 0">新建图书、生成图书等操作需要先有一个已初始化的工作区。可以切换到其他目录（支持 ~），或直接初始化当前目录。</p>` : ""}
        <div class="answer-row" style="margin-top:12px">
          <input type="text" id="ws-path" placeholder="输入新的工作区目录，如 ~/my-books">
          <button class="btn" onclick="App.selectWorkspace()">切换工作区</button>
          ${this.ws.initialized ? "" : `<button class="btn primary" onclick="App.initWorkspace()">初始化此目录</button>`}
        </div>
        <div class="muted" style="margin-top:10px;font-size:12.5px">
          也可以用环境变量 <span class="mono">JIANWU_WORKSPACE</span> 或全局配置文件
          <span class="mono">~/.config/jianwu/config.yaml</span> 的 <span class="mono">workspace:</span> 键配置（优先级：命令行 &gt; 环境变量 &gt; 配置文件 &gt; 网页设置）。
        </div>
      </div>`;
  },

  async viewDashboard() {
    this.setNav("dashboard");
    $("#view").innerHTML = SKELETON_VIEW;
    await this.refreshWs();
    const v = $("#view");
    if (!this.ws.initialized) {
      v.innerHTML = `
        <div class="page-head"><div><h1>工作区</h1><div class="desc">先配置一个工作区，才能开始创作</div></div></div>
        ${this.workspaceForm(true)}`;
      return;
    }
    const books = this.ws.books || [];
    const totalTokens = books.reduce((a, b) => a + (b.token_usage?.total_tokens || 0), 0);
    const searchCfg = this.ws.config?.search || {};
    const expandCfg = this.ws.config?.models?.expand || {};
    const stat = (num, unit, label) =>
      `<div class="stat"><div class="stat-num">${num}${unit ? `<span class="unit">${unit}</span>` : ""}</div><div class="stat-label">${label}</div></div>`;
    v.innerHTML = `
      <div class="page-head">
        <div><h1>工作区</h1><div class="desc mono">${esc(this.ws.root)}</div></div>
        <a class="btn primary" href="#/new">＋ 新建图书</a>
      </div>
      <div class="stats">
        ${stat(books.length, "本", "图书")}
        ${stat(totalTokens.toLocaleString(), "tokens", "累计 LLM 用量")}
        ${stat(esc(searchCfg.primary || "—"), "", `搜索 · reader ${esc(searchCfg.reader || "—")}`)}
        ${stat(esc(expandCfg.provider || "—"), "", `展开模型 · ${esc(expandCfg.model || "")}`)}
      </div>
      <h2>书籍</h2>
      ${books.length ? this.bookCards(books) : `<div class="card">${emptyState("冊", "还没有图书", "从一次 12 维设计访谈开始：AI 逐维推荐，你确认或修改。", `<a class="btn primary" href="#/new">＋ 新建图书</a>`)}</div>`}
      <h2>工作区设置</h2>
      ${this.workspaceForm(false)}
    `;
  },

  bookCards(books) {
    return `<div class="grid">${books.map(b => {
      const st = b.stats || {};
      const total = st.total || 1;
      const seg = (n, c) => `<span style="width:${(n / total) * 100}%;background:${c}"></span>`;
      return `
      <div class="card book-card" onclick="location.hash='#/book/${esc(b.slug)}'">
        <h3>${esc(b.title)}</h3>
        <div class="slug">${esc(b.slug)} · ${statusBadge(b.status)}</div>
        <div class="progressbar">
          ${seg(st.scaffolded || 0, STATUS_COLOR.scaffolded)}
          ${seg(st.expanded || 0, STATUS_COLOR.expanded)}
          ${seg(st.reviewed || 0, STATUS_COLOR.reviewed)}
          ${seg(st.final || 0, STATUS_COLOR.final)}
          ${seg(st.failed || 0, STATUS_COLOR.failed)}
        </div>
        <div class="ch-meta">
          ${st.total || 0} 章 · ${fmtTokens(b.token_usage)} · 更新于 ${esc(b.updated_at || "")}
        </div>
      </div>`;
    }).join("")}</div>`;
  },

  async initWorkspace() {
    try {
      await api("/workspace/init", { body: {} });
      toast("工作区已初始化", "ok");
      await this.refreshWs();
      this.route();
    } catch (e) { toast(e.message, "err"); }
  },

  async selectWorkspace() {
    const path = $("#ws-path")?.value.trim();
    if (!path) { toast("请输入工作区目录路径", "err"); return; }
    try {
      const res = await api("/workspace/select", { body: { path } });
      await this.refreshWs();
      if (res.initialized) {
        toast(`工作区已切换：${res.root}`, "ok");
      } else {
        toast(`工作区已切换到 ${res.root}；该目录尚未初始化，可点击「初始化此目录」`, "ok");
      }
      this.route();
    } catch (e) { toast(e.message, "err"); }
  },

  async viewBooks() {
    this.setNav("books");
    $("#view").innerHTML = SKELETON_VIEW;
    await this.refreshWs();
    const books = this.ws.books || [];
    $("#view").innerHTML = `
      <div class="page-head">
        <div><h1>书籍</h1><div class="desc">${books.length} 本</div></div>
        <a class="btn primary" href="#/new">＋ 新建图书</a>
      </div>
      ${books.length ? this.bookCards(books) : `<div class="card">${emptyState("冊", "还没有图书", "从一次 12 维设计访谈开始：AI 逐维推荐，你确认或修改。", `<a class="btn primary" href="#/new">＋ 新建图书</a>`)}</div>`}`;
  },

  async viewBook(slug) {
    this.setNav("books");
    $("#view").innerHTML = SKELETON_VIEW;
    const { meta, outline, stats } = await api("/books/" + encodeURIComponent(slug));
    const v = $("#view");
    this.currentBook = slug;
    const chRows = (p) => p.chapters.map(c => {
      const addr = `${String(p.index).padStart(2, "0")}-${String(c.index).padStart(2, "0")}`;
      const verdictBad = (c.verdicts || []).filter(x => !x.verified).length;
      return `
      <tr>
        <td class="addr">${addr}</td>
        <td>
          <div><a href="javascript:App.showChapter('${esc(slug)}',${p.index},${c.index})"><b>${esc(c.title)}</b></a></div>
          <div class="ch-meta">${c.word_count ? c.word_count + " 字 · " : ""}${c.citations_count || 0} 引用${c.unverified_claims ? ` · <span style="color:var(--warn)">${c.unverified_claims} 未验证</span>` : ""}${(c.verdicts || []).length ? ` · 核查 ${verdictBad ? verdictBad + " 未通过" : "全部通过"}` : ""}</div>
        </td>
        <td>${statusBadge(c.status)}</td>
        <td class="actions">
          ${c.status === "scaffolded" || c.status === "failed" ? `<button class="btn sm" onclick="App.expandChapter('${esc(slug)}',${p.index},${c.index},1)">展开</button>` : ""}
          ${c.status === "expanded" ? `<button class="btn sm" onclick="App.reviewChapter('${esc(slug)}',${p.index},${c.index})">审阅</button>` : ""}
          ${(c.status === "expanded" || c.status === "reviewed") ? `
            <button class="btn sm" onclick="App.factcheckChapter('${esc(slug)}',${p.index},${c.index})">核查</button>
            <button class="btn sm" onclick="App.reviseChapter('${esc(slug)}',${p.index},${c.index})">修订</button>` : ""}
          ${(c.status === "expanded" || c.status === "reviewed" || c.status === "final") ? `<button class="btn sm" onclick="App.expandChapter('${esc(slug)}',${p.index},${c.index},2)">重写</button>` : ""}
          <button class="btn sm danger" onclick="App.deleteChapter('${esc(slug)}',${p.index},${c.index})">删除</button>
        </td>
      </tr>`;
    }).join("");

    v.innerHTML = `
      <div class="page-head">
        <div>
          <h1>${esc(meta.title)}</h1>
          <div class="desc mono">${esc(slug)} · ${statusBadge(meta.status)} · ${esc(meta.archetype || "")} · ${fmtTokens(meta.token_usage)}</div>
        </div>
        <div class="toolbar">
          <div class="tb-group" role="group" aria-label="章节操作">
            <button class="btn primary" onclick="App.expandAll('${esc(slug)}')">展开全部</button>
            <button class="btn" onclick="App.addChapter('${esc(slug)}')">＋ 章节</button>
          </div>
          <span class="tb-sep" aria-hidden="true"></span>
          <button class="btn" onclick="App.finalize('${esc(slug)}')">定稿</button>
          <span class="tb-sep" aria-hidden="true"></span>
          <div class="tb-group" role="group" aria-label="导出">
            <span class="tb-label">导出</span>
            <button class="btn" onclick="App.exportBook('${esc(slug)}','md')">Markdown</button>
            <button class="btn" onclick="App.exportBook('${esc(slug)}','hugo')">Hugo</button>
            <button class="btn" onclick="App.exportBook('${esc(slug)}','pdf')">PDF</button>
            <button class="btn" onclick="App.downloadExport('${esc(slug)}','md')">下载 .md</button>
          </div>
        </div>
      </div>
      ${outline.parts.length === 0 ? `<div class="card">${emptyState("纲", "大纲为空", "这本书还没有目录结构。回到「新建图书」完成访谈，即可生成大纲与章节框架。", `<a class="btn" href="#/new">去新建访谈</a>`)}</div>` : ""}
      ${outline.parts.map(p => `
        <div class="part-block">
          <div class="part-title">Part ${p.index} · ${esc(p.title)} ${p.role ? `<span class="muted" style="font-weight:400;font-size:12px">(${esc(p.role)})</span>` : ""}</div>
          <table class="chapters">
            <tr><th>编号</th><th>章节</th><th>状态</th><th></th></tr>
            ${chRows(p) || `<tr><td colspan="4" class="muted" style="text-align:center">本 part 无章节</td></tr>`}
          </table>
        </div>`).join("")}
    `;
  },

  // ----- actions -----

  async startJob(path, body, after) {
    try {
      const res = await api(path, { body: body || {} });
      toast("任务已提交，可在右下角查看进度", "ok");
      this.openJobs();
      if (after) this.onJobDone = after;
      return res.job_id;
    } catch (e) { toast(e.message, "err"); }
  },

  expandChapter(slug, p, c, force) {
    this.startJob(`/books/${encodeURIComponent(slug)}/chapters/${p}/${c}/expand`, { force },
      j => { if (j.slug === slug) this.route(); });
  },
  expandAll(slug) {
    this.startJob(`/books/${encodeURIComponent(slug)}/expand-all`, { force: 0 },
      j => { if (j.slug === slug) this.route(); });
  },
  factcheckChapter(slug, p, c) {
    this.startJob(`/books/${encodeURIComponent(slug)}/chapters/${p}/${c}/factcheck`, {},
      j => { if (j.slug === slug) this.route(); });
  },
  reviseChapter(slug, p, c) {
    this.startJob(`/books/${encodeURIComponent(slug)}/chapters/${p}/${c}/revise`, {},
      j => { if (j.slug === slug) this.route(); });
  },
  async reviewChapter(slug, p, c) {
    try {
      await api(`/books/${encodeURIComponent(slug)}/chapters/${p}/${c}/review`, { body: {} });
      toast("已标记为已审阅（人工确认）", "ok");
      this.route();
    } catch (e) { toast(e.message, "err"); }
  },
  async deleteChapter(slug, p, c) {
    const addr = `${String(p).padStart(2, "0")}-${String(c).padStart(2, "0")}`;
    const ok = await Modal.open({ title: "删除章节", text: `确定删除章节 ${addr}？此操作不可撤销。`, okText: "删除", danger: true });
    if (!ok) return;
    try {
      await api(`/books/${encodeURIComponent(slug)}/chapters/${p}/${c}`, { method: "DELETE" });
      toast("章节已删除", "ok");
      this.route();
    } catch (e) { toast(e.message, "err"); }
  },
  async addChapter(slug) {
    const after = await Modal.open({ title: "插入章节", text: "插入到哪个章节之后？（如 01-02）", input: true, placeholder: "01-02", okText: "下一步" });
    if (!after) return;
    const topic = await Modal.open({ title: "插入章节", text: "新章节标题：", input: true, placeholder: "例如：时间的形状", okText: "添加" });
    if (!topic) return;
    api(`/books/${encodeURIComponent(slug)}/chapters`, { body: { after, topic } })
      .then(() => { toast("章节已添加", "ok"); this.route(); })
      .catch(e => toast(e.message, "err"));
  },
  async finalize(slug) {
    try {
      await api(`/books/${encodeURIComponent(slug)}/finalize`, { body: { dry_run: true } });
      if (!(await Modal.open({ title: "定稿确认", text: "所有章节均已审阅，确认定稿？", okText: "定稿" }))) return;
      await api(`/books/${encodeURIComponent(slug)}/finalize`, { body: {} });
      toast("图书已定稿", "ok");
      this.route();
    } catch (e) { toast(e.message, "err"); }
  },
  exportBook(slug, target) {
    this.startJob(`/books/${encodeURIComponent(slug)}/export`, { target },
      j => { if (j.slug === slug && j.status === "succeeded") toast("导出完成，可点击「下载 .md」获取文件", "ok"); });
  },
  downloadExport(slug, target) {
    window.open(`/api/v1/books/${encodeURIComponent(slug)}/export/file?target=${target}`, "_blank");
  },
  async cancelJob(id) {
    try { await api(`/jobs/${id}`, { method: "DELETE" }); toast("已请求取消"); }
    catch (e) { toast(e.message, "err"); }
  },

  // ----- chapter drawer -----

  async showChapter(slug, p, c) {
    try {
      const ch = await api(`/books/${encodeURIComponent(slug)}/chapters/${p}/${c}`);
      $("#chapter-drawer-title").innerHTML =
        `${String(p).padStart(2, "0")}-${String(c).padStart(2, "0")} ${esc(ch.title)} ${statusBadge(ch.status)}`;
      const verdicts = (ch.verdicts || []).map(v => `
        <div class="verdict ${v.verified ? "ok" : "bad"}">
          <b>${v.verified ? "✓" : "✗"}</b> ${esc(v.claim_text)}
          ${v.suggested_rewrite ? `<div class="muted">建议改写：${esc(v.suggested_rewrite)}</div>` : ""}
          <div class="muted" style="font-size:12px">${esc(v.reasoning || "")}</div>
        </div>`).join("");
      const cites = (ch.citations || []).map(x =>
        `<li>[${esc(x.id)}] <a href="${esc(x.url)}" target="_blank" rel="noopener">${esc(x.title || x.url)}</a></li>`).join("");
      $("#chapter-drawer-body").innerHTML = `
        ${ch.abstract ? `<div class="muted" style="margin-bottom:10px">${esc(ch.abstract)}</div>` : ""}
        <div class="md-body">${ch.body ? renderMD(ch.body) : `<div class="empty">尚未展开（状态：${STATUS_ZH[ch.status] || ch.status}）</div>`}</div>
        ${(ch.verdicts || []).length ? `<h2>事实核查</h2>${verdicts}` : ""}
        ${cites ? `<h2>引用来源</h2><ul class="cite-list">${cites}</ul>` : ""}
      `;
      $("#chapter-drawer").classList.remove("hidden");
      this.syncBackdrop();
    } catch (e) { toast(e.message, "err"); }
  },
  closeChapter() { $("#chapter-drawer").classList.add("hidden"); this.syncBackdrop(); },
  openJobs() { $("#jobs-drawer").classList.remove("hidden"); this.syncBackdrop(); Jobs.renderList(); },
  closeJobs() { $("#jobs-drawer").classList.add("hidden"); this.syncBackdrop(); },
  syncBackdrop() {
    const anyOpen = !$("#jobs-drawer").classList.contains("hidden")
      || !$("#chapter-drawer").classList.contains("hidden");
    $("#drawer-backdrop").classList.toggle("hidden", !anyOpen);
  },

  // ----- new book wizard (grill over HTTP) -----

  wizard: null,

  async viewNewWizard(state) {
    this.setNav("new");
    const v = $("#view");
    // Gate: book creation requires a configured + initialized workspace.
    if (!state) {
      await this.refreshWs();
      if (!this.ws.initialized) {
        v.innerHTML = `
          <div class="page-head"><div><h1>新建图书</h1><div class="desc">12 维设计访谈：LLM 逐维推荐，你确认或修改</div></div></div>
          ${this.workspaceForm(true)}`;
        return;
      }
      // Step 0: topic input.
      v.innerHTML = `
        <div class="page-head"><div><h1>新建图书</h1><div class="desc">12 维设计访谈：LLM 逐维推荐，你确认或修改</div></div></div>
        <div class="card wizard">
          <h2 style="margin-top:0">这本书要回答什么核心问题？</h2>
          <div class="answer-row">
            <input type="text" id="wiz-topic" placeholder="例如：为什么时间感知会被扭曲？我们如何与时间和解？">
            <button class="btn primary" onclick="App.wizardStart()">开始访谈</button>
          </div>
          <div class="muted" style="margin-top:10px;font-size:13px">其余 11 个维度（受众、目标、结构原型、深度、篇幅…）将逐个出现，回车或点按钮接受 AI 推荐即可。</div>
        </div>`;
      $("#wiz-topic").focus();
      $("#wiz-topic").addEventListener("keydown", e => { if (e.key === "Enter") this.wizardStart(); });
      return;
    }
    // Render current question.
    const w = state;
    const dims = w.dimensions || [];
    const stepEls = w.answered.map((_, i) => `<span class="step done"></span>`).join("");
    const remaining = w.dim ? `<span class="step"></span>` : "";
    v.innerHTML = `
      <div class="page-head"><div><h1>新建图书</h1><div class="desc">主题：${esc(w.topic)}</div></div>
        <button class="btn ghost" onclick="App.wizardAbort()">放弃</button></div>
      <div class="card wizard">
        <div class="wizard-steps">${stepEls}${remaining}</div>
        ${w.complete ? `
          <h2 style="margin-top:0">✓ 访谈完成</h2>
          <div class="kv" style="margin-bottom:16px">
            ${w.answers ? Object.entries(w.answers).map(([k, val]) => `<span class="k">${esc(k)}</span><span>${esc(val)}</span>`).join("") : ""}
          </div>
          <button class="btn primary" onclick="App.wizardGenerate()">生成大纲与章节框架 →</button>
        ` : `
          <h2 style="margin-top:0">${esc(w.dim.name)}<span class="q-dim"> · ${esc(w.dim.id)}</span></h2>
          <p style="margin:4px 0 0">${esc(w.dim.question)}</p>
          ${w.recommendation ? `<div class="reco"><div class="reco-tag">AI 推荐</div>${esc(w.recommendation)}</div>` : `<div class="reco muted">AI 推荐不可用，请手动输入</div>`}
          ${w.dim.options && w.dim.options.length ? `
            <div class="opt-chips">${w.dim.options.map(o => `<button type="button" class="chip" onclick="App.wizardPick('${esc(o)}')">${esc(o)}</button>`).join("")}</div>` : ""}
          <div class="answer-row">
            <input type="text" id="wiz-answer" placeholder="输入你的答案，留空接受推荐${w.dim.default_value ? `（skip=默认 ${esc(w.dim.default_value)}）` : ""}">
            <button class="btn primary" onclick="App.wizardSubmit()">确认</button>
            <button class="btn" onclick="App.wizardSubmit('skip')">skip</button>
          </div>
        `}
      </div>`;
    const input = $("#wiz-answer");
    if (input) {
      input.focus();
      input.addEventListener("keydown", e => { if (e.key === "Enter") this.wizardSubmit(); });
    }
  },

  async wizardStart() {
    const topic = $("#wiz-topic").value.trim();
    if (!topic) { toast("请先输入选题", "err"); return; }
    try {
      const dims = (await api("/grill/tree")).dimensions;
      const sess = await api("/grill/sessions", { body: { topic } });
      const answered = Object.keys(sess.answers || {});
      this.wizard = { sessionId: sess.session_id, topic, dimensions: dims, answered, answers: sess.answers, dim: sess.dimension, recommendation: sess.recommendation, complete: sess.complete };
      this.viewNewWizard(this.wizard);
    } catch (e) { toast(e.message, "err"); }
  },

  wizardPick(opt) {
    const input = $("#wiz-answer");
    if (input) { input.value = opt; input.focus(); }
  },

  async wizardSubmit(skip) {
    const w = this.wizard;
    if (!w) return;
    const answer = skip === "skip" ? "skip" : ($("#wiz-answer")?.value.trim() || "");
    try {
      const res = await api(`/grill/sessions/${w.sessionId}/answer`, { body: { answer } });
      w.answered.push(w.dim.id);
      w.answers = res.answers;
      w.dim = res.dimension;
      w.recommendation = res.recommendation;
      w.complete = res.complete;
      this.viewNewWizard(w);
    } catch (e) { toast(e.message, "err"); }
  },

  wizardAbort() {
    const w = this.wizard;
    if (w && w.sessionId) api(`/grill/sessions/${w.sessionId}`, { method: "DELETE" }).catch(() => {});
    this.wizard = null;
    location.hash = "#/new";
    this.route();
  },

  async wizardGenerate() {
    const w = this.wizard;
    if (!w) return;
    try {
      const res = await api(`/grill/sessions/${w.sessionId}/generate`, { body: { force: false } });
      this.wizard = null;
      toast("图书生成任务已提交", "ok");
      this.openJobs();
      const slug = res.slug;
      this.onJobDone = (j) => {
        if (j.result && j.result.slug === slug) { location.hash = `#/book/${encodeURIComponent(slug)}`; this.route(); }
      };
    } catch (e) { toast(e.message, "err"); }
  },

  // ----- corpus -----

  async viewCorpus() {
    this.setNav("corpus");
    $("#view").innerHTML = SKELETON_VIEW;
    try {
      const [stats, list] = await Promise.all([api("/corpus/stats"), api("/corpus")]);
      $("#view").innerHTML = `
        <div class="page-head">
          <div><h1>参考语料</h1><div class="desc">${stats.books} 本 · ${stats.parts} parts · ${stats.chapters} 章 · 语料来自工作区（无内置语料）</div></div>
          <div class="tb-group">
            <button class="btn primary" onclick="App.corpusCollect()">⚡ 自动采集</button>
            <button class="btn" onclick="App.corpusSync()">从目录同步</button>
            <button class="btn" onclick="App.corpusReindex()">重建 embedding 索引</button>
          </div>
        </div>
        <div class="card">
          ${list.books.length === 0 ? emptyState("料", "语料库为空", "点击「自动采集」按主题从网络采集，或「从目录同步」导入本地 JSON。语料会作为大纲访谈与展开的参考。") : `
          <table class="chapters">
            <tr><th>slug</th><th>标题</th><th>原型</th><th>结构</th></tr>
            ${list.books.map(b => `
              <tr>
                <td class="mono">${esc(b.slug)}</td>
                <td><a href="javascript:App.showCorpus('${esc(b.slug)}')">${esc(b.title_zh)}</a></td>
                <td class="mono">${esc(b.archetype || "")}</td>
                <td>${b.parts} parts / ${b.chapters} 章</td>
              </tr>`).join("")}
          </table>`}
        </div>`;
    } catch (e) {
      $("#view").innerHTML = `<div class="card"><b>语料加载失败</b><div class="muted">${esc(e.message)}</div></div>`;
    }
  },

  async showCorpus(slug) {
    try {
      const b = await api("/corpus/" + encodeURIComponent(slug));
      $("#chapter-drawer-title").textContent = b.title_zh || b.slug;
      $("#chapter-drawer-body").innerHTML = `
        <div class="kv">
          <span class="k">slug</span><span class="mono">${esc(b.slug)}</span>
          <span class="k">原型</span><span class="mono">${esc(b.archetype)}</span>
          <span class="k">受众/深度</span><span>${esc(b.audience)} / ${esc(b.depth)}</span>
          <span class="k">摘要</span><span>${esc(b.abstract || "")}</span>
        </div>
        ${(b.parts || []).map(p => `
          <h2>Part ${p.index} · ${esc(p.title_zh)}</h2>
          <ul>${(p.chapters || []).map(c => `<li>${esc(c.title_zh || c.title)}</li>`).join("")}</ul>`).join("")}
      `;
      $("#chapter-drawer").classList.remove("hidden");
      this.syncBackdrop();
    } catch (e) { toast(e.message, "err"); }
  },

  async corpusSync() {
    const from = await Modal.open({ title: "从目录同步语料", text: "语料 JSON 目录路径：", input: true, placeholder: "~/my-corpus", okText: "同步" });
    if (!from) return;
    api("/corpus/sync", { body: { from } })
      .then(r => { toast(`同步完成：${r.synced} 本${r.issues?.length ? `，${r.issues.length} 个问题` : ""}`, "ok"); this.viewCorpus(); })
      .catch(e => toast(e.message, "err"));
  },

  async corpusCollect() {
    const topic = await Modal.open({ title: "自动采集语料", text: "按主题自动搜索网页、阅读并提取结构化语料。", input: true, placeholder: "例如：时间感知与注意力", okText: "开始采集" });
    if (!topic) return;
    this.startJob("/corpus/collect", { topic },
      j => { if (j.status === "succeeded") this.viewCorpus(); });
  },

  corpusReindex() {
    this.startJob("/corpus/reindex", {});
  },

  // ----- config -----

  CFG: {
    llmProviders: ["", "gemini", "glm", "kimi", "deepseek", "ollama"],
    searchProviders: ["", "brave", "serper"],
    readerProviders: ["", "jina"],
    loggingLevels: ["", "debug", "info", "warn", "error"],
  },
  CFG_LABELS: {
    "": "（默认）", gemini: "Gemini", glm: "智谱 GLM", kimi: "Kimi", deepseek: "DeepSeek",
    ollama: "Ollama（本地）",
    brave: "Brave", serper: "Serper", jina: "Jina",
  },
  SECRET_LABELS: {
    gemini_api_key: "Gemini", glm_api_key: "智谱 GLM", kimi_api_key: "Kimi",
    deepseek_api_key: "DeepSeek", brave_api_key: "Brave 搜索",
    serper_api_key: "Serper 搜索", jina_api_key: "Jina 阅读器",
  },

  async viewConfig() {
    this.setNav("config");
    $("#view").innerHTML = SKELETON_VIEW;
    try {
      // Secrets are global; config needs an initialized workspace.
      const secrets = await api("/secrets");
      this._secretFields = (secrets.fields || []).map(f => f.field);
      let cfg = null;
      try { cfg = await api("/config"); } catch { /* workspace not initialized */ }
      await this.refreshWs();

      const L = this.CFG_LABELS;
      const opts = (list, cur, emptyLabel) => list.map(o =>
        `<option value="${esc(o)}" ${o === (cur || "") ? "selected" : ""}>${esc(o === "" ? emptyLabel : L[o] || o)}</option>`).join("");
      const stageRow = (key, label) => {
        const m = cfg.models[key] || {};
        const fb = m.fallback || {};
        return `
          <tr>
            <td>${label}</td>
            <td><select class="field" id="cfg-${key}-provider">${opts(this.CFG.llmProviders, m.provider, "（内置默认）")}</select></td>
            <td><input class="field" id="cfg-${key}-model" type="text" value="${esc(m.model || "")}" placeholder="留空用内置默认"></td>
            <td><select class="field" id="cfg-${key}-fb-provider">${opts(this.CFG.llmProviders, fb.provider, "（无）")}</select></td>
            <td><input class="field" id="cfg-${key}-fb-model" type="text" value="${esc(fb.model || "")}" placeholder="fallback 模型"></td>
            <td><input class="field num" id="cfg-${key}-timeout" type="number" min="0" value="${m.timeout || 0}"></td>
          </tr>`;
      };
      const srcInput = (id, list) =>
        `<input class="field" id="${id}" type="text" value="${esc((list || []).join(", "))}" placeholder="user, builtin">`;
      const sel = (id, list, cur, emptyLabel) =>
        `<select class="field" id="${id}">${opts(list, cur, emptyLabel)}</select>`;

      const secretRows = (secrets.fields || []).map(f => {
        const srcBadge = f.source === "env" ? `<span class="badge s-final">环境变量</span>`
          : f.source === "file" ? `<span class="badge v-ok">配置文件</span>`
          : `<span class="badge s-scaffolded">未设置</span>`;
        const editable = f.source !== "env";
        return `
          <tr>
            <td>${esc(this.SECRET_LABELS[f.field] || f.field)}<div class="ch-meta mono">${esc(f.env_var)}</div></td>
            <td>${srcBadge}</td>
            <td class="mono">${esc(f.masked) || "—"}</td>
            <td>${editable
              ? `<input class="field" id="secret-${esc(f.field)}" type="password" placeholder="输入新值；留空表示不修改" autocomplete="off">`
              : `<div class="ch-meta">由环境变量设置，网页无法修改</div>`}</td>
            <td class="actions">${f.source === "file" ? `<button class="btn sm danger" onclick="App.clearSecret('${esc(f.field)}')">清除</button>` : ""}</td>
          </tr>`;
      }).join("");

      const modelCard = cfg ? `
        <div class="card">
          <h2 style="margin-top:0">阶段模型</h2>
          <table class="chapters cfg">
            <tr><th>阶段</th><th>Provider</th><th>模型</th><th>Fallback</th><th>模型</th><th>超时(秒)</th></tr>
            ${stageRow("intake", "访谈 intake")}
            ${stageRow("outline", "大纲 outline")}
            ${stageRow("scaffolding", "框架 scaffolding")}
            ${stageRow("expand", "展开 expand")}
          </table>
          <div class="ch-meta" style="margin-top:8px">provider 留空 = 使用内置默认；超时 0 = 沿用全局超时；fallback 选「（无）」表示不配置（与主模型相同的 fallback 会被忽略）。</div>
        </div>
        <div class="card">
          <h2 style="margin-top:0">通用设置</h2>
          <div class="cfg-grid">
            <label>全局超时（秒）<span class="ch-meta">0 = 不限时</span></label>
            <input class="field" id="cfg-llm-timeout" type="number" min="0" value="${cfg.llm?.timeout || 0}">
            <label>搜索主源</label>
            ${sel("cfg-search-primary", this.CFG.searchProviders, cfg.search?.primary, "（内置默认）")}
            <label>搜索备用源</label>
            ${sel("cfg-search-fallback", this.CFG.searchProviders, cfg.search?.fallback, "（内置默认）")}
            <label>网页阅读器</label>
            ${sel("cfg-search-reader", this.CFG.readerProviders, cfg.search?.reader, "（内置默认）")}
            <label>框架并发数<span class="ch-meta">同时展开的章节数上限</span></label>
            <input class="field" id="cfg-concurrency" type="number" min="0" value="${cfg.scaffolding?.concurrency || 0}">
            <label>日志级别</label>
            ${sel("cfg-logging-level", this.CFG.loggingLevels, cfg.logging?.level, "（内置默认）")}
          </div>
        </div>
        <div class="card">
          <h2 style="margin-top:0">来源顺序</h2>
          <div class="cfg-grid">
            <label>原型库 <span class="mono">archetypes.library</span></label>${srcInput("cfg-archetypes-library", cfg.archetypes?.library)}
            <label>风格指南 <span class="mono">style.guide</span></label>${srcInput("cfg-style-guide", cfg.style?.guide)}
            <label>风格示例 <span class="mono">style.samples</span></label>${srcInput("cfg-style-samples", cfg.style?.samples)}
          </div>
          <div class="ch-meta" style="margin-top:8px">可选值：user（工作区）/ builtin（内置）；逗号分隔，从高到低排列，留空 = 使用默认。</div>
        </div>` : `
        <div class="card"><b>配置需要先初始化工作区</b>
          <div class="muted">API 密钥是全局的，可立即设置；阶段模型等配置保存在工作区目录内，请先到 <a href="#/">工作区页</a> 选择并初始化目录。</div>
        </div>`;

      $("#view").innerHTML = `
        <div class="page-head">
          <div><h1>配置</h1><div class="desc">优先级：命令行 &gt; 环境变量 &gt; 全局配置 &gt; 工作区配置（此处保存的位置）&gt; 内置默认；保存后立即生效</div></div>
          ${cfg ? `<button class="btn primary" onclick="App.saveConfig()">保存配置</button>` : ""}
        </div>
        ${modelCard}
        <div class="card">
          <h2 style="margin-top:0">API 密钥</h2>
          <div class="ch-meta" style="margin-bottom:10px">写入 <span class="mono">${esc(secrets.path || "~/.config/jianwu/secrets.yaml")}</span>（权限 0600）。环境变量优先于文件，因此环境变量已设置时文件修改不会生效。</div>
          <table class="chapters">
            <tr><th>密钥</th><th>状态</th><th>当前值</th><th style="width:34%">设置新值</th><th></th></tr>
            ${secretRows}
          </table>
          <div style="margin-top:12px"><button class="btn primary" onclick="App.saveSecrets()">保存密钥</button></div>
        </div>
        <div class="card">
          <h2 style="margin-top:0">工作区目录</h2>
          <div class="kv">
            <span class="k">当前工作区</span><span class="mono">${esc(this.ws?.root || "—")}</span>
          </div>
          <p class="ch-meta" style="margin:8px 0 0">工作区目录在 <a href="#/">工作区页</a> 切换（写入全局配置文件的 workspace 键）。</p>
        </div>`;
    } catch (e) {
      $("#view").innerHTML = `<div class="card"><b>配置加载失败</b><div class="muted">${esc(e.message)}</div></div>`;
    }
  },

  async saveConfig() {
    const val = id => ($(id)?.value ?? "").trim();
    const int = id => Math.max(0, parseInt($(id)?.value, 10) || 0);
    const list = id => val(id) ? val(id).split(/[,，]/).map(s => s.trim()).filter(Boolean) : [];
    const stages = ["intake", "outline", "scaffolding", "expand"];
    const models = {};
    for (const st of stages) {
      const m = { provider: val(`#cfg-${st}-provider`), model: val(`#cfg-${st}-model`), timeout: int(`#cfg-${st}-timeout`), fallback: null };
      const fp = val(`#cfg-${st}-fb-provider`), fm = val(`#cfg-${st}-fb-model`);
      if (fp || fm) m.fallback = { provider: fp, model: fm, timeout: 0 };
      models[st] = m;
    }
    const body = {
      llm: { timeout: int("#cfg-llm-timeout") },
      models,
      search: { primary: val("#cfg-search-primary"), fallback: val("#cfg-search-fallback"), reader: val("#cfg-search-reader") },
      archetypes: { library: list("#cfg-archetypes-library") },
      style: { guide: list("#cfg-style-guide"), samples: list("#cfg-style-samples") },
      scaffolding: { concurrency: int("#cfg-concurrency") },
      logging: { level: val("#cfg-logging-level") },
    };
    try {
      await api("/config", { body });
      toast("配置已保存到工作区 config.yaml", "ok");
      this.route();
    } catch (e) { toast(e.message, "err"); }
  },

  async saveSecrets() {
    const updates = {};
    for (const f of (this._secretFields || [])) {
      const input = $(`#secret-${f}`);
      if (input && input.value.trim() !== "") updates[f] = input.value.trim();
    }
    if (!Object.keys(updates).length) { toast("没有要保存的新密钥（输入框留空 = 不修改）", "err"); return; }
    try {
      await api("/secrets", { body: updates });
      toast("密钥已保存", "ok");
      this.route();
    } catch (e) { toast(e.message, "err"); }
  },

  async clearSecret(field) {
    const ok = await Modal.open({
      title: "清除密钥",
      text: `确定从配置文件中清除 ${this.SECRET_LABELS[field] || field} 吗？`,
      okText: "清除", danger: true,
    });
    if (!ok) return;
    try {
      await api("/secrets", { body: { [field]: "" } });
      toast("密钥已清除", "ok");
      this.route();
    } catch (e) { toast(e.message, "err"); }
  },
};

// 全局交互接线：内置对话框、Escape 关闭抽屉、遮罩点击。
$("#modal-ok").addEventListener("click", () => Modal.submit());
$("#modal-cancel").addEventListener("click", () => Modal.close(null));
$("#modal").addEventListener("click", e => { if (e.target.id === "modal") Modal.close(null); });
$("#modal-input").addEventListener("keydown", e => { if (e.key === "Enter") Modal.submit(); });
$("#drawer-backdrop").addEventListener("click", () => { App.closeChapter(); App.closeJobs(); });
document.addEventListener("keydown", e => {
  if (e.key !== "Escape") return;
  if (!$("#modal").classList.contains("hidden")) return Modal.close(null);
  if (!$("#chapter-drawer").classList.contains("hidden")) return App.closeChapter();
  if (!$("#jobs-drawer").classList.contains("hidden")) return App.closeJobs();
});

window.App = App;
App.boot();
