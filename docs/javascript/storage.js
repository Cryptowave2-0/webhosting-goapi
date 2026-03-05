function get_item(key) {
  let raw = localStorage.getItem(key);

  try {
    let data = JSON.parse(raw);
    return data;
  } catch (e) {
    if (e instanceof SyntaxError) {
      console.error("Erreur :", e);
      return -1;
    } else {
      return raw;
    }
  }
}

function save_item(key, item) {
  let data;

  try {
    data = JSON.stringify(item);
  } catch (e) {
    if (e instanceof SyntaxError) {
      console.error("Erreur :", e);
      return -1;
    } else {
      data = item;
    }
  }

  return localStorage.setItem(key, data);
}

function get_cookie(name) {
  const cookies = document.cookie.split("; ");
  console.log(cookies)
  for (let cookie of cookies) {
    const [key, value] = cookie.split("=");
    if (key === name) return value;
  }

  return null;
}