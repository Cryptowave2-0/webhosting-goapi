//  AUTH

async function login(username, password) {
  const res = await fetch(`${API_URL}/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ username, password }),
  });
  return await res.text();
}

async function logout() {
  const res = await fetch(`${API_URL}/logout`, {
    method: "POST",
    credentials: "include",
  });
  return await res.text();
}

//  SCRIPTS

async function list_scripts() {
  const res = await fetch(`${API_URL}/scripts`, {
    credentials: "include",
  });
  return await res.json();
}

async function get_script(script_id) {
  const res = await fetch(`${API_URL}/scripts/${script_id}`, {
    credentials: "include",
  });
  return await res.json();
}

async function delete_script(script_id) {
  const res = await fetch(`${API_URL}/scripts/${script_id}`, {
    method: "DELETE",
    credentials: "include",
  });
  return await res.status;
}

async function upload_script({
  name,
  description,
  language,
  entrypoint,
  files,
}) {
  const form = new FormData();
  form.append("name", name);
  form.append("language", language);
  form.append("entrypoint", entrypoint);
  if (description) form.append("description", description);

  for (const file of files) {
    form.append("file", file);
  }

  const res = await fetch(`${API_URL}/scripts/upload`, {
    method: "POST",
    credentials: "include",
    body: form,
  });
  return await res.json();
}

// EXECUTIONS

async function run_script(script_id) {
  const res = await fetch(`${API_URL}/scripts/${script_id}/run`, {
    method: "POST",
    credentials: "include",
  });
  return await res.json();
}

async function get_script_executions(script_id) {
  const res = await fetch(`${API_URL}/scripts/${script_id}/executions`, {
    credentials: "include",
  });
  console.error(res);
  return await res.json();
}

async function get_execution(execution_id) {
  const res = await fetch(`${API_URL}/executions/${execution_id}`, {
    credentials: "include",
  });
  return await res.json();
}

async function get_logs(execution_id, format = "json") {
  const url =
    format === "text"
      ? `${API_URL}/executions/${execution_id}/logs?format=text`
      : `${API_URL}/executions/${execution_id}/logs`;

  const res = await fetch(url, { credentials: "include" });
  return (await format) === "text" ? res.text() : res.json();
}


async function stream_execution_logs(execution_id, onLog, onDone, onError) {
  const url = `${API_URL}/executions/${execution_id}/stream`;

  const source = new EventSource(url, { withCredentials: true });

  source.addEventListener("log", (e) => {
    const data = JSON.parse(e.data);
    onLog(data); // { stream: "stdout" | "stderr", content: "..." }
  });

  source.addEventListener("done", (e) => {
    const data = JSON.parse(e.data);
    onDone(data); // { status: "success" | "failed", exit_code: 0 | 1 }
    source.close();
  });

  source.onerror = (e) => {
    if (onError) onError(e);
    source.close();
  };

  return source; // retourné pour pouvoir faire source.close() manuellement
}