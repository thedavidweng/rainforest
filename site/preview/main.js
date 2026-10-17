"use strict";

const params = new URLSearchParams(location.search);
const initialTheme = params.get("theme") || "dark";
document.documentElement.setAttribute("data-mygo-theme", initialTheme);

window.addEventListener("message", (event) => {
  if (event.data && event.data.type === "theme" && (event.data.theme === "dark" || event.data.theme === "light")) {
    document.documentElement.setAttribute("data-mygo-theme", event.data.theme);
  }
});

const status = document.getElementById("status");
const chinese = params.get("lang") === "zh-Hans";
document.documentElement.lang = chinese ? "zh-Hans" : "en";
status.textContent = chinese ? "正在加载 Rainforest…" : "Loading Rainforest…";
const go = new Go();
WebAssembly.instantiateStreaming(fetch("app.wasm"), go.importObject)
  .then(({ instance }) => go.run(instance))
  .catch((error) => {
    status.textContent = chinese ? "预览加载失败，请刷新重试。" : "Preview failed to load. Please reload.";
    console.error(error);
  });
new MutationObserver((_, observer) => {
  if (document.querySelector("#mygo canvas")) {
    status.hidden = true;
    observer.disconnect();
  }
}).observe(document.getElementById("mygo"), { childList: true, subtree: true });
