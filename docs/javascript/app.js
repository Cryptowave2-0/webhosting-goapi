async function __init__() {
  const session_token = get_cookie("session_token");
  console.log(session_token)

  if (session_token == null) {
    render(login_page());
  } else {
    render(await scripts_page());
  }
}

function render(component) {
  document.getElementById("root").innerHTML = component;
}

__init__();
