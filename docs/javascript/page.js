function login_page() {
  return `
    <div>

        <label for="username">
            Username:
        </label>
        <input type="text" id="username" name="username" 
            placeholder="Enter your Username" required>

        <label for="password">
            Password:
        </label>
        <input type="password" id="password" name="password" 
            placeholder="Enter your Password" required>

        <button type="button" onclick="handle_login()">Se connecter</button>
    </div>
    `;
}

function logout_page() {
  return `
    <div>
        <button type="button" onclick="handle_logout()">Se déconnecter</button>
    </div>
    `;
}

async function scripts_page() {
  const scripts = await list_scripts();
  console.log(scripts);

  const scripts_html = scripts
    .map(
      (script) => `
        <li class="script" onclick="handle_select('${script.id}')" style="cursor: pointer;">
            <h3>${script.name}</h3>
            <p>${script.description}</p>
            <span>${script.language} ${script.created_at}</span>
        </li>
    `,
    )
    .join("");

  console.log(scripts_html);
  return `<ul>${scripts_html}</ul>`;
}

async function select_page(res) {
  const executions = await get_script_executions(res.id);
  console.log(executions);

  const executions_html = executions
    .map(
      (execution) => `
        <li class="script" onclick="handle_select_execution('${execution.id}', '${res.id}')" style="cursor: pointer;">
            <h3>${execution.id}</h3>
            <p>${execution.exit_code}: ${execution.status}</p>
            <span>from ${execution.started_at} to ${execution.created_at}</span>
        </li>
    `,
    )
    .join("");
  return `${res.name}
  <button type="button" onclick="handle_back_to_scripts_page()">back</button>
  <div>${res.tree.map((execution) => `<span>${execution.path} - ${execution.size}</span>`)}</div>
  <ul>${executions_html}</ul>`;
}

async function select_execution_page(res, script_id) {
  const logsEl = `<pre id="logs"></pre>`;

  if (current_stream) {
    current_stream.close();
    current_stream = null;
  }

  // Stream les logs en direct
  current_stream = stream_execution_logs(
    res.id,
    (log) => {
      const pre = document.getElementById("logs");
      if (!pre) return;
      const line = document.createElement("span");
      line.style.color = color(log);
      line.textContent = log.content + "\n";
      pre.appendChild(line);
    },
    (result) => {
      const pre = document.getElementById("logs");
      if (!pre) return;
      const line = document.createElement("span");
      line.style.color = "blue";
      line.textContent = `\n[${result.status} — exit code: ${result.exit_code}]`;
      pre.appendChild(line);
      current_stream = null;
    },
    (err) => {
      console.error("Stream error:", err);
      current_stream = null;
    },
  );

  return `
  <button type="button" onclick="handle_select('${script_id}')">back</button>
  <div>${logsEl}</div>`;
}

function color(log) {
  if (log.stream === "stdout") return "black";
  if (log.stream === "stderr") return "red";
  return "blue";
}