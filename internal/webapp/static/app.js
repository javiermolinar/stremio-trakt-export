"use strict";

const translations = {
  en: {
    eyebrow: "LOCAL MIGRATION TOOL", subtitle: "Migrate Stremio Not Watched items and watched history to Trakt.", reset: "Reset session",
    connectStremio: "Connect Stremio", email: "Email", password: "Password", signIn: "Sign in",
    stremioHint: "Credentials go directly from this local server to Stremio. The resulting token is saved locally with user-only permissions.",
    authKeyAlternative: "Or use an existing authKey", useAuthKey: "Use authKey", connectTrakt: "Connect Trakt",
    setupTitle: "First-time Trakt app setup",
    setupCreate: 'Open <a href="https://app.trakt.tv/settings/apps/api/new" target="_blank" rel="noopener noreferrer">Trakt → Create API App</a>. Sign in with your normal Trakt account if asked.',
    setupName: "Enter any app name, for example <strong>Stremio Export</strong>. Leave the other options as they are.",
    setupRedirect: "In Trakt’s <strong>Redirect URI</strong> field, paste exactly:", copyUri: "Copy URI", copied: "Copied",
    setupClient: 'Save the app. Open it from <a href="https://app.trakt.tv/settings/apps/api" target="_blank" rel="noopener noreferrer">Trakt → Your API Apps</a> and copy its <strong>Client ID</strong> into the field below.',
    setupSecret: 'Use the <strong>Client ID</strong>, not the Client Secret. This app uses PKCE, so newer Trakt apps correctly show “Client Secret: Not issued.”',
    clientIdLabel: "Paste the app’s Client ID", clientIdPlaceholder: "Client ID from your Trakt API app", redirectLabel: "Redirect URI used in the app",
    redirectHint: "This must match the Trakt app exactly, including <strong>https://</strong> and <strong>/callback</strong>.", continueTrakt: "Continue to Trakt login",
    oauthOneBefore: "Open the", oauthLink: "regular Trakt login", oauthOneAfter: "and click Authorize.",
    oauthTwo: "<strong>2.</strong> Trakt redirects to <code>example.com/callback</code>. The page may show an error or not load; this is expected.",
    oauthThree: "<strong>3.</strong> Copy the <strong>entire URL</strong> from that page’s address bar. It must include <code>code=...</code> and <code>state=...</code>. Paste it below—not just the code.",
    callbackLabel: "Callback URL", completeSignIn: "Complete Trakt sign-in", loadLibrary: "Load your library", fetchData: "Fetch Stremio data",
    chooseMigration: "Choose what to migrate", watchlistTab: "Not Watched → Trakt List", historyTab: "Watched → History",
    selectAll: "Select all", clear: "Clear", filterPlaceholder: "Filter titles", titleColumn: "Title", typeColumn: "Type",
    importTitle: "Import selected items", importButton: "Import into Trakt", footer: "Runs only on this machine. Tokens survive restarts; Reset session deletes the saved credential file.",
    notConnected: "Not connected", connected: "Connected", readyAuthorize: "Ready to authorize", configuredSession: "Configured for this session",
    connecting: "Connecting…", stremioConnected: "Stremio connected. Fetch your library to choose items.", preparingLogin: "Preparing login…",
    openTraktStatus: "Open the regular Trakt login, authorize, then paste the callback URL back here.", completingSignIn: "Completing sign-in…",
    traktConnected: "Trakt connected. You can import selected items.", fetching: "Fetching…",
    loadedItems: "Loaded {watchlist} Not Watched items and {history} watched items.", destinationList: "Add to Trakt List", destinationHistory: "Add to Trakt History",
    statsList: "{movies} Not Watched movies · {shows} Not Watched shows", statsHistory: "{movies} watched movies · {shows} watched shows · {episodes} episodes",
    historyHint: "Watched history includes movies and episodes, even if removed from My List. Selecting a show includes all its watched episodes. Episode dates are approximate; existing Trakt History entries are skipped.",
    listHint: "This mirrors Stremio’s Not Watched classification using watched flags and episode bitfields. Removed and temporary entries are excluded; playback progress alone does not exclude an item.",
    show: "Show", movie: "Movie", episodeCount: "{count} episodes", selectedList: "{count} Not Watched personal-list items selected.",
    selectedHistory: "{count} watched History items selected.", issues: "{count} conversion issues",
    destinationListConfirm: "the Trakt personal list ‘Stremio My List’", destinationHistoryConfirm: "Trakt History",
    confirmImport: "Import {count} selected items into {destination}? Existing entries will be skipped.", importing: "Importing…",
    importFinished: "Import finished. Review the result below; rerun safely if any items were not found.", resetConfirm: "Clear the saved session and tokens?",
    languageAria: "Switch to Spanish", copyFallback: "Redirect URI selected. Press Ctrl+C or ⌘C to copy it.", migrationType: "Migration type",
    stremioHintMemory: "Credentials go directly from this local server to Stremio. On this platform, tokens stay in memory and are cleared when the app stops.",
    footerMemory: "Runs only on this machine. On Windows, tokens stay in memory and are cleared when the app stops.",
  },
  es: {
    eyebrow: "HERRAMIENTA DE MIGRACIÓN LOCAL", subtitle: "Migra tus pelis y series de Stremio a Trakt.tv.", reset: "Restablecer sesión",
    connectStremio: "Conectar Stremio", email: "Correo electrónico", password: "Contraseña", signIn: "Iniciar sesión",
    stremioHint: "Las credenciales van directamente de este servidor local a Stremio. El token resultante se guarda localmente con permisos solo para tu usuario.",
    authKeyAlternative: "O usa una authKey existente", useAuthKey: "Usar authKey", connectTrakt: "Conectar Trakt",
    setupTitle: "Configuración inicial de la app de Trakt",
    setupCreate: 'Abre <a href="https://app.trakt.tv/settings/apps/api/new" target="_blank" rel="noopener noreferrer">Trakt → Crear app de API</a>. Inicia sesión con tu cuenta habitual de Trakt si se solicita.',
    setupName: "Escribe cualquier nombre para la app, por ejemplo <strong>Stremio Export</strong>. Deja las demás opciones como están.",
    setupRedirect: "En el campo <strong>Redirect URI</strong> de Trakt, pega exactamente:", copyUri: "Copiar URI", copied: "Copiada",
    setupClient: 'Guarda la app. Ábrela desde <a href="https://app.trakt.tv/settings/apps/api" target="_blank" rel="noopener noreferrer">Trakt → Tus apps de API</a> y copia su <strong>Client ID</strong> en el campo siguiente.',
    setupSecret: 'Usa el <strong>Client ID</strong>, no el Client Secret. Esta app utiliza PKCE, por lo que es correcto que las apps nuevas de Trakt muestren “Client Secret: Not issued”.',
    clientIdLabel: "Pega el Client ID de la app", clientIdPlaceholder: "Client ID de tu app de API de Trakt", redirectLabel: "Redirect URI usada en la app",
    redirectHint: "Debe coincidir exactamente con la app de Trakt, incluidos <strong>https://</strong> y <strong>/callback</strong>.", continueTrakt: "Continuar al inicio de sesión de Trakt",
    oauthOneBefore: "Abre el", oauthLink: "inicio de sesión normal de Trakt", oauthOneAfter: "y pulsa Autorizar.",
    oauthTwo: "<strong>2.</strong> Trakt redirige a <code>example.com/callback</code>. La página puede mostrar un error o no cargar; es normal.",
    oauthThree: "<strong>3.</strong> Copia la <strong>URL completa</strong> de la barra de direcciones. Debe incluir <code>code=...</code> y <code>state=...</code>. Pégala abajo, no copies solo el código.",
    callbackLabel: "URL de callback", completeSignIn: "Completar inicio de sesión de Trakt", loadLibrary: "Carga tu biblioteca", fetchData: "Obtener datos de Stremio",
    chooseMigration: "Elige qué migrar", watchlistTab: "No vistos → Lista de Trakt", historyTab: "Vistos → Historial",
    selectAll: "Seleccionar todo", clear: "Limpiar", filterPlaceholder: "Filtrar títulos", titleColumn: "Título", typeColumn: "Tipo",
    importTitle: "Importar elementos seleccionados", importButton: "Importar a Trakt", footer: "Se ejecuta solo en este equipo. Los tokens sobreviven a los reinicios; Restablecer sesión elimina el archivo de credenciales.",
    notConnected: "No conectado", connected: "Conectado", readyAuthorize: "Listo para autorizar", configuredSession: "Configurado para esta sesión",
    connecting: "Conectando…", stremioConnected: "Stremio conectado. Obtén tu biblioteca para elegir elementos.", preparingLogin: "Preparando inicio de sesión…",
    openTraktStatus: "Abre el inicio de sesión normal de Trakt, autoriza y pega aquí la URL de callback.", completingSignIn: "Completando inicio de sesión…",
    traktConnected: "Trakt conectado. Ya puedes importar los elementos seleccionados.", fetching: "Obteniendo datos…",
    loadedItems: "Se cargaron {watchlist} elementos no vistos y {history} elementos vistos.", destinationList: "Añadir a la lista de Trakt", destinationHistory: "Añadir al historial de Trakt",
    statsList: "{movies} películas no vistas · {shows} series no vistas", statsHistory: "{movies} películas vistas · {shows} series vistas · {episodes} episodios",
    historyHint: "El historial incluye películas y episodios aunque se hayan eliminado de Mi lista. Al seleccionar una serie se incluyen todos sus episodios vistos. Las fechas de los episodios son aproximadas; se omiten las entradas que ya existen en el historial de Trakt.",
    listHint: "Se reproduce la clasificación No vistos de Stremio mediante sus marcas y campos de episodios vistos. Se excluyen los elementos eliminados y temporales; el progreso de reproducción por sí solo no excluye un elemento.",
    show: "Serie", movie: "Película", episodeCount: "{count} episodios", selectedList: "{count} elementos no vistos seleccionados para la lista personal.",
    selectedHistory: "{count} elementos vistos seleccionados para el historial.", issues: "{count} problemas de conversión",
    destinationListConfirm: "la lista personal de Trakt «Stremio My List»", destinationHistoryConfirm: "el historial de Trakt",
    confirmImport: "¿Importar {count} elementos seleccionados a {destination}? Se omitirán las entradas existentes.", importing: "Importando…",
    importFinished: "Importación terminada. Revisa el resultado; puedes volver a ejecutarla de forma segura si no se encontró algún elemento.", resetConfirm: "¿Borrar la sesión y los tokens guardados?",
    languageAria: "Cambiar a inglés", copyFallback: "Redirect URI seleccionada. Pulsa Ctrl+C o ⌘C para copiarla.", migrationType: "Tipo de migración",
    stremioHintMemory: "Las credenciales van directamente de este servidor local a Stremio. En esta plataforma, los tokens permanecen en memoria y se borran al cerrar la app.",
    footerMemory: "Se ejecuta solo en este equipo. En Windows, los tokens permanecen en memoria y se borran al cerrar la app.",
  },
};

const browserSpanish = typeof navigator.language === "string" && navigator.language.toLowerCase().startsWith("es");
let savedLanguage = "";
try { savedLanguage = localStorage.getItem("stremio-trakt-language") || ""; } catch { /* Browser storage may be disabled. */ }
const state = {
  csrf: "", stremio: false, trakt: false, traktConfigured: false, persistentCredentials: false,
  items: [], historyItems: [], stats: {}, view: "watchlist", issueCount: 0,
  language: savedLanguage === "es" || savedLanguage === "en" ? savedLanguage : (browserSpanish ? "es" : "en"),
  selections: {
    watchlist: { movies: new Set(), shows: new Set() },
    history: { movies: new Set(), shows: new Set() },
  },
};
const $ = (id) => document.getElementById(id);
function t(key, values = {}) {
  let text = (translations[state.language] && translations[state.language][key]) || translations.en[key] || key;
  for (const [name, value] of Object.entries(values)) text = text.replaceAll(`{${name}}`, String(value));
  return text;
}
function applyLanguage() {
  document.documentElement.lang = state.language;
  for (const element of document.querySelectorAll("[data-i18n]")) element.textContent = t(element.dataset.i18n);
  for (const element of document.querySelectorAll("[data-i18n-html]")) element.innerHTML = t(element.dataset.i18nHtml);
  for (const element of document.querySelectorAll("[data-i18n-placeholder]")) element.placeholder = t(element.dataset.i18nPlaceholder);
  $("language-toggle").textContent = state.language === "en" ? "ES" : "EN";
  $("language-toggle").setAttribute("aria-label", t("languageAria"));
  $("filter").setAttribute("aria-label", t("filterPlaceholder"));
  document.querySelector(".view-tabs").setAttribute("aria-label", t("migrationType"));
  updateCredentialText();
  updateConnections();
  if (!$("preview-section").hidden) renderCurrentView();
  if (state.issueCount > 0) $("issues-summary").textContent = t("issues", { count: state.issueCount });
}

function updateCredentialText() {
  document.querySelector('[data-i18n="stremioHint"]').textContent = state.persistentCredentials ? t("stremioHint") : t("stremioHintMemory");
  document.querySelector("footer").textContent = state.persistentCredentials ? t("footer") : t("footerMemory");
}

async function api(path, options = {}) {
  const request = { ...options, headers: { ...(options.headers || {}) } };
  if (request.method && request.method !== "GET") request.headers["X-CSRF-Token"] = state.csrf;
  if (request.body) request.headers["Content-Type"] = "application/json";
  const response = await fetch(path, request);
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || `${response.status} ${response.statusText}`);
  return data;
}
function showError(error) {
  $("status").hidden = true;
  $("error").textContent = error instanceof Error ? error.message : String(error);
  $("error").hidden = false;
}
function showStatus(message) { $("error").hidden = true; $("status").textContent = message; $("status").hidden = false; }
function clearNotices() { $("error").hidden = true; $("status").hidden = true; }
function setBusy(button, busy, label) {
  if (busy) { button.dataset.label = button.textContent; button.textContent = label; }
  else if (button.dataset.label) button.textContent = button.dataset.label;
  button.disabled = busy;
}
function updateConnections() {
  for (const [name, connected] of [["stremio", state.stremio], ["trakt", state.trakt]]) {
    const badge = $(`${name}-badge`);
    badge.textContent = connected ? t("connected") : t("notConnected");
    badge.classList.toggle("connected", connected);
  }
  if (!state.trakt && state.traktConfigured) $("trakt-badge").textContent = t("readyAuthorize");
  const configured = state.traktConfigured;
  $("trakt-setup-guide").open = !configured;
  $("trakt-client-id").required = !configured;
  $("trakt-client-id").placeholder = configured ? t("configuredSession") : t("clientIdPlaceholder");
  $("load-library").disabled = !state.stremio;
  $("import-button").disabled = !state.trakt || selectedCount() === 0;
}

async function loadSession() {
  const session = await api("/api/session");
  state.csrf = session.csrf;
  state.stremio = session.stremio_connected;
  state.trakt = session.trakt_connected;
  state.traktConfigured = session.trakt_configured;
  state.persistentCredentials = session.persistent_credentials;
  updateCredentialText();
  updateConnections();
  if (session.has_preview) await loadPreview("GET");
}
async function loginStremio(event, authKey = false) {
  event.preventDefault();
  clearNotices();
  const button = event.target.querySelector("button[type=submit]");
  setBusy(button, true, t("connecting"));
  try {
    const body = authKey ? { auth_key: $("stremio-authkey").value } : {
      email: $("stremio-email").value, password: $("stremio-password").value,
    };
    await api("/api/stremio/login", { method: "POST", body: JSON.stringify(body) });
    state.stremio = true;
    $("stremio-password").value = "";
    $("stremio-authkey").value = "";
    $("preview-section").hidden = true;
    $("import-section").hidden = true;
    state.items = [];
    state.historyItems = [];
    for (const view of ["watchlist", "history"]) {
      state.selections[view].movies.clear();
      state.selections[view].shows.clear();
    }
    updateConnections();
    showStatus(t("stremioConnected"));
  } catch (error) { showError(error); }
  finally { setBusy(button, false); updateConnections(); }
}
async function startTrakt(event) {
  event.preventDefault();
  clearNotices();
  setBusy($("trakt-connect"), true, t("preparingLogin"));
  try {
    const clientID = $("trakt-client-id").value.trim();
    const redirectURI = $("trakt-redirect-uri").value.trim();
    const authorization = await api("/api/trakt/start", { method: "POST", body: JSON.stringify({ client_id: clientID, redirect_uri: redirectURI }) });
    if (clientID) state.trakt = false;
    state.traktConfigured = true;
    $("trakt-client-id").value = "";
    $("authorization-link").href = authorization.authorization_url;
    $("oauth-flow").hidden = false;
    $("callback-url").value = "";
    updateConnections();
    showStatus(t("openTraktStatus"));
  } catch (error) { showError(error); }
  finally { setBusy($("trakt-connect"), false); }
}
async function completeTrakt(event) {
  event.preventDefault();
  clearNotices();
  setBusy($("callback-submit"), true, t("completingSignIn"));
  try {
    await api("/api/trakt/exchange", { method: "POST", body: JSON.stringify({ callback_url: $("callback-url").value.trim() }) });
    state.trakt = true;
    $("oauth-flow").hidden = true;
    $("callback-url").value = "";
    updateConnections();
    showStatus(t("traktConnected"));
  } catch (error) { showError(error); }
  finally { setBusy($("callback-submit"), false); }
}
async function loadPreview(method = "POST") {
  clearNotices();
  setBusy($("load-library"), true, t("fetching"));
  try {
    const preview = await api("/api/preview", method === "POST" ? { method, body: "{}" } : {});
    state.items = preview.items || [];
    state.historyItems = preview.history_items || [];
    renderPreview(preview);
    showStatus(t("loadedItems", { watchlist: state.items.length, history: state.historyItems.length }));
  } catch (error) { showError(error); }
  finally { setBusy($("load-library"), false); updateConnections(); }
}

function currentItems() { return state.view === "history" ? state.historyItems : state.items; }
function itemGroup(item) { return item.type === "series" ? "shows" : "movies"; }
function checkbox(item) {
  const input = document.createElement("input");
  input.type = "checkbox";
  input.dataset.id = item.id;
  input.setAttribute("aria-label", `${state.view} ${item.name}`);
  input.checked = state.selections[state.view][itemGroup(item)].has(item.id);
  input.addEventListener("change", () => {
    const selected = state.selections[state.view][itemGroup(item)];
    if (input.checked) selected.add(item.id);
    else selected.delete(item.id);
    updateSelectionSummary();
  });
  return input;
}
function renderPreview(preview) {
  $("preview-section").hidden = false;
  $("import-section").hidden = false;
  state.stats = preview.stats;
  const issues = preview.issues || [];
  state.issueCount = issues.length;
  $("issues-wrap").hidden = issues.length === 0;
  $("issues-summary").textContent = t("issues", { count: issues.length });
  $("issues").replaceChildren();
  for (const issue of issues) {
    const li = document.createElement("li");
    li.textContent = `${issue.name || issue.id}: ${issue.reason}`;
    $("issues").appendChild(li);
  }
  renderCurrentView();
}
function renderCurrentView() {
  const history = state.view === "history";
  for (const view of ["watchlist", "history"]) {
    $(`tab-${view}`).classList.toggle("active", state.view === view);
    $(`tab-${view}`).setAttribute("aria-selected", state.view === view ? "true" : "false");
  }
  $("destination-heading").textContent = history ? t("destinationHistory") : t("destinationList");
  $("preview-stats").textContent = history
    ? t("statsHistory", { movies: state.stats.movies || 0, shows: state.historyItems.filter((item) => item.type === "series").length, episodes: state.stats.episodes || 0 })
    : t("statsList", { movies: state.stats.watchlist_movies || 0, shows: state.stats.watchlist_shows || 0 });
  $("view-hint").textContent = history ? t("historyHint") : t("listHint");
  const body = $("items");
  body.replaceChildren();
  for (const item of currentItems()) {
    const row = document.createElement("tr");
    row.dataset.search = item.name.toLowerCase();
    const title = document.createElement("td");
    title.textContent = item.name;
    const id = document.createElement("small");
    id.textContent = ` ${item.id}`;
    title.appendChild(id);
    const type = document.createElement("td");
    type.textContent = item.type === "series" ? t("show") : t("movie");
    const destination = document.createElement("td");
    destination.appendChild(checkbox(item));
    if (history && item.type === "series") {
      const count = document.createElement("small");
      count.textContent = ` ${t("episodeCount", { count: item.watched_count })}`;
      destination.appendChild(count);
    }
    row.append(title, type, destination);
    body.appendChild(row);
  }
  updateSelectionSummary();
}
function currentSelection() {
  const selection = { history_movies: [], history_shows: [], watchlist_movies: [], watchlist_shows: [] };
  selection[`${state.view}_movies`] = [...state.selections[state.view].movies];
  selection[`${state.view}_shows`] = [...state.selections[state.view].shows];
  return selection;
}
function selectedCount() { return state.selections[state.view].movies.size + state.selections[state.view].shows.size; }
function updateSelectionSummary() {
  const count = selectedCount();
  $("selection-summary").textContent = state.view === "history" ? t("selectedHistory", { count }) : t("selectedList", { count });
  $("import-button").disabled = !state.trakt || count === 0;
}
function setAvailable(value) {
  for (const item of currentItems()) {
    if (value) state.selections[state.view][itemGroup(item)].add(item.id);
    else state.selections[state.view][itemGroup(item)].delete(item.id);
  }
  renderCurrentView();
}
async function importSelected() {
  const selection = currentSelection();
  const destination = state.view === "history" ? t("destinationHistoryConfirm") : t("destinationListConfirm");
  if (!window.confirm(t("confirmImport", { count: selectedCount(), destination }))) return;
  clearNotices();
  setBusy($("import-button"), true, t("importing"));
  $("import-result").hidden = true;
  try {
    const result = await api("/api/import", { method: "POST", body: JSON.stringify(selection) });
    $("import-result").textContent = JSON.stringify(result, null, 2);
    $("import-result").hidden = false;
    showStatus(t("importFinished"));
  } catch (error) { showError(error); }
  finally { setBusy($("import-button"), false); updateConnections(); }
}

$("language-toggle").addEventListener("click", () => {
  state.language = state.language === "en" ? "es" : "en";
  try { localStorage.setItem("stremio-trakt-language", state.language); } catch { /* Non-essential preference. */ }
  applyLanguage();
});
$("stremio-form").addEventListener("submit", (event) => loginStremio(event));
$("authkey-form").addEventListener("submit", (event) => loginStremio(event, true));
$("trakt-form").addEventListener("submit", startTrakt);
$("callback-form").addEventListener("submit", completeTrakt);
$("copy-redirect").addEventListener("click", async () => {
  const uri = "https://example.com/callback";
  $("trakt-redirect-uri").value = uri;
  try {
    await navigator.clipboard.writeText(uri);
    $("copy-redirect").textContent = t("copied");
    setTimeout(() => { $("copy-redirect").textContent = t("copyUri"); }, 1500);
  } catch {
    $("trakt-redirect-uri").focus();
    $("trakt-redirect-uri").select();
    showStatus(t("copyFallback"));
  }
});
$("load-library").addEventListener("click", () => loadPreview());
$("all-watchlist").addEventListener("click", () => setAvailable(true));
$("select-none").addEventListener("click", () => setAvailable(false));
$("tab-watchlist").addEventListener("click", () => { state.view = "watchlist"; $("filter").value = ""; renderCurrentView(); });
$("tab-history").addEventListener("click", () => { state.view = "history"; $("filter").value = ""; renderCurrentView(); });
$("filter").addEventListener("input", (event) => {
  const query = event.target.value.toLowerCase();
  for (const row of document.querySelectorAll("tbody tr")) row.hidden = !row.dataset.search.includes(query);
});
$("import-button").addEventListener("click", importSelected);
$("logout").addEventListener("click", async () => {
  if (!window.confirm(t("resetConfirm"))) return;
  await api("/api/logout", { method: "POST", body: "{}" }).catch(showError);
  window.location.reload();
});

applyLanguage();
loadSession().catch(showError);
