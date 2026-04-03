// ============================================================
//  UTILS — fonctions partagées entre toutes les pages
// ============================================================

const Utils = {
  // ── Auth ────────────────────────────────────────────────
  requireAuth() {
    // Vérifie si on est connecté via un cookie visible ou un flag localStorage
    const loggedIn = localStorage.getItem("orbit_logged_in");
    if (!loggedIn) {
      window.location.href = "/webhosting-goapi/login";
      return false;
    }
    return true;
  },

  requireGuest() {
    const loggedIn = localStorage.getItem("orbit_logged_in");
    if (loggedIn) {
      window.location.href = "/webhosting-goapi/dashboard";
      return false;
    }
    return true;
  },

  setLoggedIn(username) {
    localStorage.setItem("orbit_logged_in", "true");
    localStorage.setItem("orbit_username", username);
  },

  setLoggedOut() {
    localStorage.removeItem("orbit_logged_in");
    localStorage.removeItem("orbit_username");
  },

  getUsername() {
    return localStorage.getItem("orbit_username") || "Utilisateur";
  },

  isLoggedIn() {
    return !!localStorage.getItem("orbit_logged_in");
  },

  // ── Navigation ──────────────────────────────────────────
  go(url) {
    console.log(url)
    window.location.href = url;
  },

  // ── Toast notifications ─────────────────────────────────
  toast(message, type = "info", duration = 3500) {
    const container = document.getElementById("toast-container") || (() => {
      const div = document.createElement("div");
      div.id = "toast-container";
      document.body.appendChild(div);
      return div;
    })();

    const toast = document.createElement("div");
    toast.className = `toast toast-${type}`;
    toast.innerHTML = `
      <span class="toast-icon">${{ info: "◎", success: "◉", error: "✕", warn: "△" }[type] || "◎"}</span>
      <span>${message}</span>
    `;
    container.appendChild(toast);
    requestAnimationFrame(() => toast.classList.add("show"));
    setTimeout(() => {
      toast.classList.remove("show");
      setTimeout(() => toast.remove(), 400);
    }, duration);
  },

  // ── Formatage ────────────────────────────────────────────
  formatDate(isoString) {
    if (!isoString) return "—";
    return new Date(isoString).toLocaleString("fr-FR", {
      day: "2-digit", month: "2-digit", year: "numeric",
      hour: "2-digit", minute: "2-digit",
    });
  },

  formatDuration(start, end) {
    if (!start || !end) return "—";
    const ms = new Date(end) - new Date(start);
    if (ms < 1000) return `${ms}ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
    return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`;
  },

  formatBytes(bytes) {
    if (!bytes) return "0 B";
    const units = ["B", "KB", "MB", "GB"];
    let i = 0;
    while (bytes >= 1024 && i < units.length - 1) { bytes /= 1024; i++; }
    return `${bytes.toFixed(1)} ${units[i]}`;
  },

  statusDot(status) {
    const map = {
      running: "dot-running",
      success: "dot-success",
      failed: "dot-failed",
      pending: "dot-pending",
    };
    return `<span class="dot ${map[status] || "dot-pending"}"></span>`;
  },

  // ── Préférences utilisateur ──────────────────────────────
  getPrefs() {
    const defaults = {
      brightness: 100,
      accentColor: "#00d4ff",
      terminalFont: "JetBrains Mono",
      animationsEnabled: true,
      compactMode: false,
    };
    try {
      return { ...defaults, ...JSON.parse(localStorage.getItem("orbit_prefs") || "{}") };
    } catch { return defaults; }
  },

  savePrefs(prefs) {
    localStorage.setItem("orbit_prefs", JSON.stringify(prefs));
    Utils.applyPrefs(prefs);
  },

  applyPrefs(prefs) {
    prefs = prefs || Utils.getPrefs();
    document.documentElement.style.setProperty("--accent", prefs.accentColor);
    document.documentElement.style.setProperty("--accent-glow", prefs.accentColor + "44");
    document.body.style.filter = `brightness(${prefs.brightness}%)`;
    if (prefs.compactMode) document.body.classList.add("compact");
    else document.body.classList.remove("compact");
  },
};

window.Utils = Utils;

// Appliquer les prefs au chargement
document.addEventListener("DOMContentLoaded", () => Utils.applyPrefs());
