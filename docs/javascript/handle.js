async function handle_login() {
  const username = document.getElementById("username").value;
  const password = document.getElementById("password").value;

  const res = await login(username, password);
  console.log(username, password, res);
  save_item("res", res);

  if (res.includes("Logged in") || res.includes("success")) {
    return render(await scripts_page());
  } else {
    console.error("Login error", res.status);
  }
}

async function handle_logout() {
  const res = await logout();
  console.log(res);
  return render(login_page());
}

async function handle_select(id) {
  const res = await get_script(id);

  console.log(res);

  if (res.id == id) {
    return render(await select_page(res));
  } else {
    console.error("Select Error", res.status);
  }
}

async function handle_select_execution(id, script_id) {
  const res = await get_execution(id);

  console.log(res);

  if (res.id == id) {
    return render(await select_execution_page(res, script_id));
  } else {
    console.error("Select Error", res.status);
  }
}

function handle_back_script(script_id) {
  if (current_stream) {
    current_stream.close();
    current_stream = null;
  }
  handle_select(script_id);
}

async function handle_scripts_page() {
  render(await scripts_page())
}

function handle_back_to_scripts_page() {
  handle_scripts_page();
}