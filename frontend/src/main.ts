import { LitElement, html, nothing } from "lit";
import { activeSession, historySessions, mockSessions } from "./mock-data";
import type { ChargingSession, MeterSample, SessionStatus } from "./types";

const statusLabel = (status: SessionStatus) => status[0].toUpperCase() + status.slice(1);

class LightDomElement extends LitElement {
  protected createRenderRoot(): HTMLElement | DocumentFragment {
    return this;
  }
}

class EvStatusBadge extends LightDomElement {
  static properties = { status: { type: String } };
  status: SessionStatus = "unknown";
  render() { return html`<span class="status ${this.status}">${this.status === "charging" ? "ϟ " : ""}${statusLabel(this.status)}</span>`; }
}
customElements.define("ev-status-badge", EvStatusBadge);

class EvChart extends LightDomElement {
  static properties = { samples: { type: Array }, showPower: { type: Boolean }, showEnergy: { type: Boolean } };
  samples: MeterSample[] = [];
  showPower = true;
  showEnergy = true;
  path(values: number[], maxY: number) {
    return values.map((value, index) => {
      const x = 36 + (index / Math.max(values.length - 1, 1)) * 828;
      const y = 182 - (value / maxY) * 148;
      return `${index ? "L" : "M"}${x.toFixed(1)},${y.toFixed(1)}`;
    }).join(" ");
  }
  render() {
    const powerPath = this.path(this.samples.map((sample) => sample.powerKw), 10);
    const energyPath = this.path(this.samples.map((sample) => sample.energyKwh), 16);
    return html`<div class="chart-wrap"><svg viewBox="0 0 900 220" role="img" aria-label="Power and energy chart">
      <defs><linearGradient id="fill" x1="0" x2="0" y1="0" y2="1"><stop offset="0" stop-color="#1e88ff" stop-opacity=".24"/><stop offset="1" stop-color="#1e88ff" stop-opacity=".03"/></linearGradient></defs>
      ${[0,1,2,3,4].map((i) => html`<line x1="36" x2="864" y1="${34+i*37}" y2="${34+i*37}" stroke="var(--line)"/><text x="8" y="${38+i*37}" class="chart-label">${10-i*2}</text><text x="872" y="${38+i*37}" class="chart-label energy">${16-i*4}</text>`)}
      <text x="36" y="24" class="chart-label">Power (kW)</text><text x="805" y="24" class="chart-label energy">Energy (kWh)</text>
      ${this.showPower ? html`<path d="${powerPath} L864,182 L36,182 Z" fill="url(#fill)"/><path d="${powerPath}" fill="none" stroke="#1e88ff" stroke-width="3"/>` : nothing}
      ${this.showEnergy ? html`<path d="${energyPath}" fill="none" stroke="#42d64b" stroke-width="3" stroke-dasharray="4 4"/>` : nothing}
      <text x="36" y="207" fill="var(--muted)">Start</text><text x="416" y="207" fill="var(--muted)">Middle</text><text x="810" y="207" fill="var(--muted)">End</text>
    </svg></div>`;
  }
}
customElements.define("ev-chart", EvChart);

class EvApp extends LightDomElement {
  static properties = { route: { type: String }, selectedId: { type: String }, theme: { type: String }, search: { type: String }, status: { type: String }, sort: { type: String }, page: { type: Number }, showPower: { type: Boolean }, showEnergy: { type: Boolean }, rawOpen: { type: Boolean }, meterOpen: { type: Boolean }, copied: { type: Boolean }, lastUpdated: { type: String } };
  route = this.getAttribute("route") ?? "overview";
  selectedId = historySessions[0].id;
  theme = new URLSearchParams(location.search).get("theme") === "light" ? "light" : "dark";
  search = "";
  status = "all";
  sort = "newest";
  page = 1;
  showPower = true;
  showEnergy = true;
  rawOpen = false;
  meterOpen = false;
  copied = false;
  lastUpdated = "11:24:30 AM";
  connectedCallback() { super.connectedCallback(); this.applyTheme(); }
  updated(changed: Map<string, unknown>) { if (changed.has("theme")) this.applyTheme(); }
  applyTheme() { document.documentElement.dataset.theme = this.theme; }
  get filtered() {
    const term = this.search.toLowerCase();
    const rows = historySessions.filter((session) => (this.status === "all" || session.status === this.status) && [session.id, session.chargerId, session.chargerName, session.connectorId, session.evseId].some((value) => value.toLowerCase().includes(term)));
    return rows.sort((a, b) => this.sort === "oldest" ? a.startedAt.localeCompare(b.startedAt) : b.startedAt.localeCompare(a.startedAt));
  }
  navigate(session: ChargingSession) { this.selectedId = session.id; this.route = "details"; history.replaceState(null, "", "session.html"); }
  refresh() { this.lastUpdated = new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" }); }
  copyId(session: ChargingSession) { navigator.clipboard?.writeText(session.id).catch(() => undefined); this.copied = true; setTimeout(() => { this.copied = false; }, 1200); }
  renderShell(content: unknown) {
    return html`<div class="app-shell"><aside class="sidebar"><div class="sidebar-title"><span>☰</span><strong>Home Assistant</strong></div><nav><a><span>▦</span>Overview</a><a><span>ϟ</span>Energy</a><a><span>▣</span>Map</a><a><span>☷</span>Logbook</a><a><span>▥</span>History</a><a><span>▣</span>Media</a><a class="active"><span>ϟ</span>EV Charging</a></nav><nav class="sidebar-bottom"><a><span>●</span>Notifications <b class="pill warn">2</b></a><a><span class="avatar">JD</span>John Doe</a></nav></aside><main class="content">${content}</main></div>`;
  }
  renderOverview() {
    const pageRows = this.filtered.slice((this.page - 1) * 5, this.page * 5);
    return this.renderShell(html`<header class="mobile-bar"><button>☰</button><strong>Charging Sessions</strong><button @click=${this.refresh}>↻</button></header><header class="page-header"><h1>Charging Sessions</h1><div class="header-actions"><button class="icon-button" @click=${this.refresh}>↻</button><span class="muted">Last updated: ${this.lastUpdated}</span><button class="theme-toggle" @click=${() => this.theme = this.theme === "dark" ? "light" : "dark"}>${this.theme === "dark" ? "Light" : "Dark"} mode</button></div></header>
      <section class="panel active-session"><h2><span class="dot success"></span>Active Session</h2><div class="active-grid"><div class="session-title-block"><ev-status-badge status=${activeSession.status}></ev-status-badge><h3>${activeSession.chargerName}</h3><p>${activeSession.chargerId}</p></div><dl class="summary-list"><div><dt>Started</dt><dd>${activeSession.startedAt}<small>(${activeSession.elapsed} ago)</small></dd></div><div><dt>Elapsed</dt><dd>${activeSession.elapsed}</dd></div><div><dt>Power</dt><dd class="good">${activeSession.powerKw?.toFixed(2)} kW</dd></div><div><dt>Energy</dt><dd>${activeSession.energyKwh.toFixed(2)} kWh</dd></div></dl><dl class="identity-list"><div><dt>EVSE ID</dt><dd>${activeSession.evseId}</dd></div><div><dt>Connector ID</dt><dd>${activeSession.connectorId}</dd></div><div><dt>Meter ID</dt><dd>${activeSession.meterId}</dd></div></dl><button class="button details-link" @click=${() => this.navigate(historySessions[0])}>View session details <span>›</span></button></div><ev-chart .samples=${activeSession.samples}></ev-chart></section>
      <section class="panel history-panel"><h2>Session History</h2><div class="filters desktop-filters"><label><span>⌕</span><input .value=${this.search} @input=${(event: InputEvent) => { this.search = (event.target as HTMLInputElement).value; this.page = 1; }} placeholder="Search by session ID or charger ID" /></label><button>▣ May 12 - May 19, 2024</button><button>All Chargers⌄</button><select @change=${(event: Event) => { this.status = (event.target as HTMLSelectElement).value; this.page = 1; }}><option value="all">All Statuses</option><option value="completed">Completed</option><option value="stopped">Stopped</option><option value="interrupted">Interrupted</option><option value="unknown">Unknown</option></select><select @change=${(event: Event) => this.sort = (event.target as HTMLSelectElement).value}><option value="newest">Sort: Newest</option><option value="oldest">Sort: Oldest</option></select><button @click=${this.refresh}>↻</button></div><div class="filters mobile-filters"><button>⌕</button><button>▣</button><button>◉</button><button>▽</button><button>≡</button></div><div class="table-wrap"><table><thead><tr><th>Started ↓</th><th>Duration</th><th>Charger</th><th>Connector</th><th>Energy (kWh)</th><th>Status</th><th>Details</th></tr></thead><tbody>${pageRows.map((session) => html`<tr @click=${() => this.navigate(session)}><td>${session.startedAt}</td><td>${session.duration}</td><td>${session.chargerName}<span class="sub">${session.chargerId}</span></td><td>${session.connectorName}<span class="sub">${session.connectorId}</span></td><td>${session.energyKwh.toFixed(2)}</td><td><ev-status-badge status=${session.status}></ev-status-badge></td><td class="chev">›</td></tr>`)}</tbody></table></div><div class="mobile-cards">${pageRows.map((session) => html`<button class="session-card" @click=${() => this.navigate(session)}><span><strong>${session.startedAt}</strong><small>${session.chargerName} • ${session.connectorName}<br>${session.duration} • ${session.energyKwh.toFixed(2)} kWh</small></span><ev-status-badge status=${session.status}></ev-status-badge><span>›</span></button>`)}</div><footer class="history-footer"><span>Showing ${pageRows.length} of ${this.filtered.length} sessions</span><nav class="pagination"><button @click=${() => this.page = Math.max(1, this.page - 1)}>‹</button><button class="active">${this.page}</button><button @click=${() => this.page += 1}>›</button></nav><button class="load-more" @click=${() => this.page += 1}>Load more</button></footer></section>`);
  }
  renderDetails() {
    const session = mockSessions.find((item) => item.id === this.selectedId) ?? historySessions[0];
    const samples = session.samples.slice(this.showPower || this.showEnergy ? 0 : session.samples.length);
    return this.renderShell(html`<header class="mobile-bar"><button @click=${() => this.route = "overview"}>‹</button><strong>Charging Sessions</strong><button @click=${this.refresh}>↻</button></header><header class="page-header detail-header"><button class="back-link" @click=${() => this.route = "overview"}>‹ Charging Sessions</button><div class="title-line"><h1>Session Details</h1><ev-status-badge status=${session.status}></ev-status-badge></div><button class="theme-toggle" @click=${() => this.theme = this.theme === "dark" ? "light" : "dark"}>${this.theme === "dark" ? "Light" : "Dark"} mode</button></header><div class="subhead"><span>▣ ${session.startedAt}</span><span>›</span><span>Session ID: ${session.shortId}</span><button @click=${() => this.copyId(session)}>${this.copied ? "Copied" : "▣"}</button></div><section class="metric-grid">${[["ϟ","Energy Consumed",`${session.energyKwh.toFixed(2)} kWh`,"green"],["◔","Duration",session.duration,"blue"],["⌁","Start Reading",`${session.startReadingKwh.toFixed(2)} kWh`,"green"],["⌁","End Reading",`${(session.endReadingKwh ?? session.startReadingKwh).toFixed(2)} kWh`,"green"]].map(([icon,label,value,color]) => html`<article class="metric"><span class="metric-icon ${color}">${icon}</span><div><p>${label}</p><strong>${value}</strong></div></article>`)}<article class="metric wide"><span class="metric-icon blue">▥</span><div><p>Charger</p><strong>${session.chargerName}</strong><small>${session.chargerId}</small></div></article><article class="metric wide"><span class="metric-icon blue">●</span><div><p>Connector</p><strong>${session.connectorName}</strong><small>${session.connectorId}</small></div></article></section><section class="panel chart-panel"><div class="panel-header"><h2>Energy and Power</h2><div class="chart-controls"><label><input type="checkbox" .checked=${this.showPower} @change=${(event: Event) => this.showPower = (event.target as HTMLInputElement).checked} /> Power (kW)</label><label><input type="checkbox" .checked=${this.showEnergy} @change=${(event: Event) => this.showEnergy = (event.target as HTMLInputElement).checked} /> Energy (kWh)</label><button>1h</button><button>3h</button><button class="active">Full</button><button @click=${() => { this.showPower = true; this.showEnergy = true; }}>Reset zoom</button></div></div><ev-chart .samples=${samples} .showPower=${this.showPower} .showEnergy=${this.showEnergy}></ev-chart></section><section class="detail-grid"><article class="panel key-values"><h2>Details</h2><dl>${[["Charger ID",session.chargerId],["EVSE ID",session.evseId],["Connector ID",session.connectorId],["Meter ID",session.meterId],["Started at",session.startedAt],["Ended at",session.endedAt ?? "-"],["Session state",statusLabel(session.status)],["Full Session ID",session.id]].map(([k,v]) => html`<div><dt>${k}</dt><dd>${v}</dd></div>`)}</dl></article><article class="panel timeline"><h2>Event Timeline</h2><ol>${session.events.map((event) => html`<li><span class="node ${event.kind === "completed" ? "done" : event.kind === "meter_updates" ? "dots" : event.kind === "stopped" ? "stop" : "play"}">${event.kind === "completed" ? "✓" : event.kind === "meter_updates" ? "•••" : event.kind === "stopped" ? "■" : "▶"}</span><time>${event.timestamp}</time><p>${event.sampleCount ? html`<button class="link-button" @click=${() => this.meterOpen = !this.meterOpen}>${this.meterOpen ? "Hide" : "Show"} ${event.sampleCount} samples</button>` : event.label}<small>${event.reason ? `Reason: ${event.reason}` : ""}</small></p></li>`)}</ol></article></section><details class="panel raw-data" ?open=${this.rawOpen} @toggle=${(event: Event) => this.rawOpen = (event.target as HTMLDetailsElement).open}><summary>Raw data</summary><pre>${JSON.stringify(session, null, 2)}</pre></details>`);
  }
  render() { return this.route === "details" ? this.renderDetails() : this.renderOverview(); }
}
customElements.define("ev-app", EvApp);

class EvComponentGallery extends LightDomElement {
  static properties = { theme: { type: String } };
  theme = "dark";
  updated() { document.documentElement.dataset.theme = this.theme; }
  connectedCallback() { super.connectedCallback(); document.documentElement.dataset.theme = this.theme; }
  render() { return html`<main class="content gallery"><header class="page-header"><h1>Component Gallery</h1><button class="theme-toggle" @click=${() => this.theme = this.theme === "dark" ? "light" : "dark"}>${this.theme === "dark" ? "Light" : "Dark"} mode</button></header><section class="panel"><h2>Status Badges</h2><div class="header-actions">${["charging","completed","stopped","interrupted","unknown"].map((status) => html`<ev-status-badge status=${status as SessionStatus}></ev-status-badge>`)}</div></section><section class="metric-grid"><article class="metric"><span class="metric-icon green">ϟ</span><div><p>Large Energy Value</p><strong>123456.78 kWh</strong></div></article><article class="metric wide"><span class="metric-icon blue">▥</span><div><p>Long Charger Name</p><strong>Charger With A Very Long Display Name For Overflow Validation</strong><small>wallbox-long-identifier-001</small></div></article></section><section class="panel"><h2>Chart States</h2><ev-chart .samples=${mockSessions[1].samples}></ev-chart><ev-chart .samples=${[]} .showPower=${false} .showEnergy=${false}></ev-chart></section></main>`; }
}
customElements.define("ev-component-gallery", EvComponentGallery);
