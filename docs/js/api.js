// ============================================================
//  API CLIENT — toutes les requêtes vers le backend Go
// ============================================================

const API = {
  // ── Auth ────────────────────────────────────────────────
  async login(username, password) {
    const res = await fetch(`${CONFIG.API_URL}/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password }),
      credentials: "include",
    });
    const text = await res.text();
    if (!res.ok) throw new Error(text || "Erreur de connexion");
    return text;
  },

  async logout() {
    const res = await fetch(`${CONFIG.API_URL}/logout`, {
      method: "POST",
      credentials: "include",
    });
    return res.ok;
  },

  // ── Scripts ─────────────────────────────────────────────
  async listScripts() {
    const res = await fetch(`${CONFIG.API_URL}/scripts`, {
      credentials: "include",
    });
    if (!res.ok) throw new Error("Non autorisé");
    return res.json();
  },

  async getScript(id) {
    const res = await fetch(`${CONFIG.API_URL}/scripts/${id}`, {
      credentials: "include",
    });
    if (!res.ok) throw new Error("Script introuvable");
    return res.json();
  },

  async runScript(id) {
    const res = await fetch(`${CONFIG.API_URL}/scripts/${id}/run`, {
      method: "POST",
      credentials: "include",
    });
    if (!res.ok) throw new Error("Impossible de lancer le script");
    return res.json();
  },

  async deleteScript(id) {
    const res = await fetch(`${CONFIG.API_URL}/scripts/${id}`, {
      method: "DELETE",
      credentials: "include",
    });
    return res.ok;
  },

  async uploadScript(formData) {
    const res = await fetch(`${CONFIG.API_URL}/scripts/upload`, {
      method: "POST",
      credentials: "include",
      body: formData,
    });
    if (!res.ok) throw new Error("Échec de l'upload");
    return res.json();
  },

  // ── Executions ──────────────────────────────────────────
  async getExecutions(scriptId) {
    const res = await fetch(
      `${CONFIG.API_URL}/scripts/${scriptId}/executions`,
      {
        credentials: "include",
      },
    );
    if (!res.ok) throw new Error("Erreur exécutions");
    return res.json();
  },

  async getExecution(execId) {
    const res = await fetch(`${CONFIG.API_URL}/executions/${execId}`, {
      credentials: "include",
    });
    if (!res.ok) throw new Error("Exécution introuvable");
    return res.json();
  },

  async getLogs(execId) {
    const res = await fetch(`${CONFIG.API_URL}/executions/${execId}/logs`, {
      credentials: "include",
    });
    if (!res.ok) throw new Error("Logs introuvables");
    return res.json();
  },

  // SSE — retourne un EventSource
  streamLogs(execId) {
    // Note: les cookies sont envoyés automatiquement avec les credentials
    return new EventSource(`${CONFIG.API_URL}/executions/${execId}/stream`, {
      withCredentials: true,
    });
  },

  // ── Files ───────────────────────────────────────────────
  // Lire le contenu d'un fichier (pour l'éditeur)
  async readFile(scriptId, filePath) {
    const res = await fetch(
      `${CONFIG.API_URL}/scripts/${scriptId}/files/${filePath}`,
      {
        credentials: "include",
      },
    );
    if (!res.ok) throw new Error(await res.text());
    return res.json(); // { path, content, size }
  },

  // Sauvegarder le contenu d'un fichier
  async writeFile(scriptId, filePath, content) {
    const res = await fetch(
      `${CONFIG.API_URL}/scripts/${scriptId}/files/${filePath}`,
      {
        method: "PUT",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content }),
      },
    );
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  },

  // Créer un nouveau fichier
  async createFile(scriptId, filePath, content = "") {
    const res = await fetch(
      `${CONFIG.API_URL}/scripts/${scriptId}/files/${filePath}`,
      {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content }),
      },
    );
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  },

  // Supprimer un fichier
  async deleteFile(scriptId, filePath) {
    const res = await fetch(
      `${CONFIG.API_URL}/scripts/${scriptId}/files/${filePath}`,
      {
        method: "DELETE",
        credentials: "include",
      },
    );
    if (!res.ok) throw new Error(await res.text());
    return res.ok;
  },

  // URL de téléchargement d'un fichier (ouvrir dans nouvel onglet)
  fileDownloadUrl(scriptId, filePath) {
    return `${CONFIG.API_URL}/scripts/${scriptId}/download/${filePath}`;
  },

  // URL de téléchargement de l'archive complète
  archiveDownloadUrl(scriptId) {
    return `${CONFIG.API_URL}/scripts/${scriptId}/archive`;
  },

  // ── Settings ────────────────────────────────────────────
  async getSettings(scriptId) {
    const res = await fetch(`${CONFIG.API_URL}/scripts/${scriptId}/settings`, {
      credentials: "include",
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json(); // { settings, files }
  },

  async updateSettings(scriptId, body) {
    const res = await fetch(`${CONFIG.API_URL}/scripts/${scriptId}/settings`, {
      method: "PATCH",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  },

  // ── GitHub ──────────────────────────────────────────────
  async githubClone(scriptId, url, token = "") {
    const res = await fetch(
      `${CONFIG.API_URL}/scripts/${scriptId}/github/clone`,
      {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url, token }),
      },
    );
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  },

  async githubPull(scriptId) {
    const res = await fetch(
      `${CONFIG.API_URL}/scripts/${scriptId}/github/pull`,
      {
        method: "POST",
        credentials: "include",
      },
    );
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  },

  // ── Move file ───────────────────────────────────────────
  async moveFile(scriptId, fromPath, toPath) {
    const res = await fetch(
      `${CONFIG.API_URL}/scripts/${scriptId}/files/${fromPath}`,
      {
        method: "PATCH",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ destination: toPath }),
      },
    );
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  },
};

window.API = API;
