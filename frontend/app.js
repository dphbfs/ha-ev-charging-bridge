const sessions = [
  ["May 19, 2024 9:15 AM", "2h 09m", "Charger 1", "wallbox-1", "Connector 1", "conn-1", "12.47", "completed"],
  ["May 18, 2024 6:42 PM", "1h 18m", "Charger 1", "wallbox-1", "Connector 1", "conn-1", "7.98", "completed"],
  ["May 18, 2024 2:14 PM", "0h 24m", "Charger 2", "wallbox-2", "Connector 1", "conn-1", "1.87", "stopped"],
  ["May 17, 2024 8:05 PM", "3h 42m", "Charger 1", "wallbox-1", "Connector 1", "conn-1", "14.97", "completed"],
  ["May 17, 2024 11:11 AM", "0h 10m", "Charger 2", "wallbox-2", "Connector 1", "conn-1", "0.62", "interrupted"],
  ["May 16, 2024 9:30 PM", "4h 05m", "Charger 1", "wallbox-1", "Connector 1", "conn-1", "15.21", "completed"],
  ["May 16, 2024 7:08 AM", "0h 48m", "Charger 2", "wallbox-2", "Connector 1", "conn-1", "3.21", "unknown"],
  ["May 17, 2024 5:46 PM", "2h 27m", "Charger With A Long Display Name", "wallbox-long-identifier-001", "Long Connector Name", "connector-long-01", "9.34", "completed"],
];

function setTheme(theme) {
  document.documentElement.dataset.theme = theme;
  document.querySelectorAll(".theme-toggle").forEach((button) => {
    button.textContent = theme === "dark" ? "Light mode" : "Dark mode";
  });
}

function initTheme() {
  const params = new URLSearchParams(location.search);
  setTheme(params.get("theme") === "light" ? "light" : "dark");
  document.querySelectorAll(".theme-toggle").forEach((button) => {
    button.addEventListener("click", () => setTheme(document.documentElement.dataset.theme === "dark" ? "light" : "dark"));
  });
}

function renderSessions() {
  const rows = document.getElementById("sessionRows");
  const cards = document.getElementById("sessionCards");
  if (!rows || !cards) return;
  rows.innerHTML = sessions.map(([started, duration, charger, chargerId, connector, connectorId, energy, status]) => `
    <tr>
      <td>${started}</td><td>${duration}</td><td>${charger}<span class="sub">${chargerId}</span></td><td>${connector}<span class="sub">${connectorId}</span></td><td>${energy}</td><td><span class="status ${status}">${status[0].toUpperCase() + status.slice(1)}</span></td><td class="chev">›</td>
    </tr>`).join("");
  cards.innerHTML = sessions.slice(0, 5).map(([started, duration, charger, , connector, , energy, status]) => `
    <a class="session-card" href="./session.html">
      <span><strong>${started}</strong><small>${charger} • ${connector}<br>${duration} • ${energy} kWh</small></span><span class="status ${status}">${status[0].toUpperCase() + status.slice(1)}</span><span>›</span>
    </a>`).join("");
}

function chartPath(points, width, height, maxY) {
  return points.map((point, index) => {
    const x = 36 + (index / (points.length - 1)) * (width - 72);
    const y = height - 28 - (point / maxY) * (height - 62);
    return `${index ? "L" : "M"}${x.toFixed(1)},${y.toFixed(1)}`;
  }).join(" ");
}

function renderChart(el, detail = false) {
  const power = detail ? [0, 7.8, 8.7, 8.3, 8.5, 8.1, 6.4, 6.8, 7.9, 8.0, 7.3, 7.5, 7.2, 6.8, 6.1, 5.9, 4.2, 2.8, 0] : [0, 6.1, 6.3, 6.0, 6.7, 6.6, 5.8, 5.5, 6.0, 6.2, 6.4, 6.1, 6.7, 6.3, 6.0, 6.1, 5.9];
  const energy = power.map((_, index) => (index / (power.length - 1)) * (detail ? 12.47 : 12.47));
  const viewBox = "0 0 900 220";
  const pPath = chartPath(power, 900, 220, 10);
  const ePath = chartPath(energy, 900, 220, 16);
  el.innerHTML = `<svg viewBox="${viewBox}" role="img" aria-label="Power and energy chart">
    <defs><linearGradient id="fill" x1="0" x2="0" y1="0" y2="1"><stop offset="0" stop-color="#1e88ff" stop-opacity=".24"/><stop offset="1" stop-color="#1e88ff" stop-opacity=".03"/></linearGradient></defs>
    ${[0,1,2,3,4].map(i => `<line x1="36" x2="864" y1="${34+i*37}" y2="${34+i*37}" stroke="var(--line)"/><text x="8" y="${38+i*37}" class="chart-label">${10-i*2}</text><text x="872" y="${38+i*37}" class="chart-label energy">${16-i*4}</text>`).join("")}
    ${[0,1,2,3].map(i => `<line x1="${118+i*205}" x2="${118+i*205}" y1="34" y2="182" stroke="var(--line)"/>`).join("")}
    <text x="36" y="24" class="chart-label">Power (kW)</text><text x="805" y="24" class="chart-label energy">Energy (kWh)</text>
    <path d="${pPath} L864,182 L36,182 Z" fill="url(#fill)"/><path d="${pPath}" fill="none" stroke="#1e88ff" stroke-width="3"/><path d="${ePath}" fill="none" stroke="#42d64b" stroke-width="3" stroke-dasharray="4 4"/>
    <text x="36" y="207" fill="var(--muted)">${detail ? "2:30 PM" : "9:15 AM"}</text><text x="416" y="207" fill="var(--muted)">${detail ? "4:00 PM" : "10:15 AM"}</text><text x="810" y="207" fill="var(--muted)">${detail ? "5:28 PM" : "11:15 AM"}</text>
  </svg>`;
}

initTheme();
renderSessions();
document.querySelectorAll("[data-chart]").forEach((el) => renderChart(el, el.dataset.chart === "detail"));
