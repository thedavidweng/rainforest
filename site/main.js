"use strict";

const REPO = "thedavidweng/rainforest";
const RELEASES = `https://github.com/${REPO}/releases/latest`;
const reducedMotion = matchMedia("(prefers-reduced-motion: reduce)");

document.documentElement.classList.add("js");

const ICONS = {
  trash: "M216,48H176V40a24,24,0,0,0-24-24H104A24,24,0,0,0,80,40v8H40a8,8,0,0,0,0,16h8V208a16,16,0,0,0,16,16H192a16,16,0,0,0,16-16V64h8a8,8,0,0,0,0-16ZM96,40a8,8,0,0,1,8-8h48a8,8,0,0,1,8,8v8H96Zm96,168H64V64H192ZM112,104v64a8,8,0,0,1-16,0V104a8,8,0,0,1,16,0Zm48,0v64a8,8,0,0,1-16,0V104a8,8,0,0,1,16,0Z",
  cloudCheck: "M160,40A88.09,88.09,0,0,0,81.29,88.67,64,64,0,1,0,72,216h88a88,88,0,0,0,0-176Zm0,160H72a48,48,0,0,1,0-96c1.1,0,2.2,0,3.29.11A88,88,0,0,0,72,128a8,8,0,0,0,16,0,72,72,0,1,1,72,72Zm37.66-93.66a8,8,0,0,1,0,11.32l-48,48a8,8,0,0,1-11.32,0l-24-24a8,8,0,0,1,11.32-11.32L144,148.69l42.34-42.35A8,8,0,0,1,197.66,106.34Z",
  dashed: "M96.26,37.05A8,8,0,0,1,102,27.29a104.11,104.11,0,0,1,52,0,8,8,0,0,1-2,15.75,8.15,8.15,0,0,1-2-.26,88.09,88.09,0,0,0-44,0A8,8,0,0,1,96.26,37.05ZM53.79,55.14a104.05,104.05,0,0,0-26,45,8,8,0,0,0,15.42,4.27,88,88,0,0,1,22-38.09A8,8,0,0,0,53.79,55.14ZM43.21,151.55a8,8,0,1,0-15.42,4.28,104.12,104.12,0,0,0,26,45,8,8,0,0,0,11.41-11.22A88.14,88.14,0,0,1,43.21,151.55ZM150,213.22a88,88,0,0,1-44,0,8,8,0,1,0-4,15.49,104.11,104.11,0,0,0,52,0,8,8,0,0,0-4-15.49ZM222.65,146a8,8,0,0,0-9.85,5.58,87.91,87.91,0,0,1-22,38.08,8,8,0,1,0,11.42,11.21,104,104,0,0,0,26-45A8,8,0,0,0,222.65,146Zm-9.86-41.54a8,8,0,0,0,15.42-4.28,104,104,0,0,0-26-45,8,8,0,1,0-11.41,11.22A88,88,0,0,1,212.79,104.45Z",
  notch: "M232,128a104,104,0,0,1-208,0c0-41,23.81-78.36,60.66-95.27a8,8,0,0,1,6.68,14.54C60.15,61.59,40,93.27,40,128a88,88,0,0,0,176,0c0-34.73-20.15-66.41-51.34-80.73a8,8,0,0,1,6.68-14.54C208.19,49.64,232,87,232,128Z",
  warning: "M128,24A104,104,0,1,0,232,128,104.11,104.11,0,0,0,128,24Zm0,192a88,88,0,1,1,88-88A88.1,88.1,0,0,1,128,216Zm-8-80V80a8,8,0,0,1,16,0v56a8,8,0,0,1-16,0Zm20,36a12,12,0,1,1-12-12A12,12,0,0,1,140,172Z",
  check: "M173.66,98.34a8,8,0,0,1,0,11.32l-56,56a8,8,0,0,1-11.32,0l-24-24a8,8,0,0,1,11.32-11.32L112,148.69l50.34-50.35A8,8,0,0,1,173.66,98.34ZM232,128A104,104,0,1,1,128,24,104.11,104.11,0,0,1,232,128Zm-16,0a88,88,0,1,0-88,88A88.1,88.1,0,0,0,216,128Z",
};

const svg = (d, cls = "") => `<svg viewBox="0 0 256 256" aria-hidden="true"${cls ? ` class="${cls}"` : ""}><path d="${d}"/></svg>`;

/* ---------- i18n ---------- */

const STRINGS = {
  en: {
    "skip": "Skip to content",
    "nav.features": "Features",
    "nav.safety": "Safety",
    "nav.docs": "Docs",
    "hero.kicker": "Unofficial desktop client for Amazon Photos",
    "hero.title": "A <em>quiet</em> way to keep your photos in the cloud.",
    "hero.lede": "Upload once, back up folders one way, watch the queue, restore verified originals. A small native app — not another gallery.",
    "hero.source": "View source",
    "dl.generic": "Download",
    "dl.mac": "Download for macOS",
    "dl.windows": "Download for Windows",
    "dl.linux": "Download for Linux",
    "dl.all": "All downloads",
    "dl.note": "macOS · Windows · Linux — free and open source",
    "dl.noteMac": "Apple silicon · <a href=\"{intel}\">Intel build</a> · <a href=\"{all}\">other platforms</a>",
    "dl.noteOther": "{arch} · <a href=\"{all}\">other platforms</a>",
    "features.eyebrow": "Four things",
    "features.title": "It does four things, <em>carefully.</em>",
    "features.lede": "No timeline, no albums to manage, no AI search. Just getting files safely up — and back down when you ask.",
    "p1.title": "Upload",
    "p1.body": "Drop files or folders. See counts, sizes and what will be skipped before a single byte leaves your disk.",
    "p2.title": "Backup",
    "p2.body": "Point it at a folder. New and changed files go up, immediately or on an interval. Subfolders are mirrored.",
    "p3.title": "Activity",
    "p3.body": "A persistent queue you can pause, resume, cancel and retry. It survives quitting, crashes and sleep.",
    "p4.title": "Restore",
    "p4.body": "Bring originals back by history, folder or album. Every file is checked by size and MD5 before it lands.",
    "oneway.eyebrow": "One-way backup",
    "oneway.title": "Water only flows <em>one way.</em>",
    "oneway.lede": "New photos travel up. Deleting something on your computer never deletes it from the cloud. Try it.",
    "oneway.local": "This computer",
    "oneway.cloud": "Amazon Photos",
    "oneway.add": "Take a photo",
    "oneway.hint": "Delete a local file, or take a new photo.",
    "oneway.deleted": "{name} deleted locally. The cloud copy stays.",
    "oneway.uploaded": "{name} backed up.",
    "oneway.full": "That’s plenty of photos for a demo.",
    "oneway.synced": "Backed up",
    "oneway.kept": "Kept",
    "oneway.uploading": "Uploading…",
    "oneway.delete": "Delete {name} from this computer",
    "queue.eyebrow": "Transfer queue",
    "queue.title": "Interrupted? It <em>checks</em> before it re-sends.",
    "queue.lede": "If the app quits mid-upload, nothing is blindly sent twice. On the next launch Rainforest compares what reached the cloud by name and MD5, confirms it, and retries only what didn’t make it.",
    "q.idle": "{n} files queued",
    "q.running": "Uploading · {done} of {n} confirmed",
    "q.quit": "App quit mid-transfer",
    "q.reconcile": "Reopened · checking the cloud",
    "q.done": "All {n} files confirmed",
    "q.start": "Start",
    "q.quitAction": "Quit mid-upload",
    "q.replay": "Replay",
    "q.queued": "Queued",
    "q.unconfirmed": "Unconfirmed",
    "q.checking": "Comparing MD5…",
    "q.verified": "MD5 match · not re-sent",
    "q.retry": "Not in cloud · retrying",
    "q.confirmed": "Confirmed",
    "details.eyebrow": "Small things",
    "details.title": "Made of <em>small</em> decisions.",
    "d1.t": "External drives are respected",
    "d1.b": "Unplug a backup source and the rule pauses as “source unavailable” instead of treating your library as deleted.",
    "d2.t": "Half-written files wait",
    "d2.b": "A file still being copied or exported is left alone until its size settles.",
    "d3.t": "Folders keep their shape",
    "d3.b": "Local subfolders are mirrored under the destination. Albums stay flat, as they should.",
    "d4.t": "Your session stays local",
    "d4.b": "Sign in through Amazon’s own window. Cookies are stored in a permission-locked local vault and redacted from every log.",
    "d5.t": "English and 简体中文",
    "d5.b": "Follows your system language, or pick one from Settings or the View menu.",
    "d6.t": "Native on every desktop",
    "d6.b": "macOS, Windows and Linux on Intel and ARM. A single Go binary, no Electron.",
    "close.title": "Let it <em>rain</em> upward.",
    "close.lede": "Free, open source under AGPL-3.0, and small enough to forget it’s running.",
    "footer.disclaimer": "Rainforest is an unofficial client. It is not affiliated with, endorsed by, or sponsored by Amazon.com, Inc.",
    "footer.changelog": "Changelog",
    "footer.security": "Security",
  },
  zh: {
    "skip": "跳到正文",
    "nav.features": "功能",
    "nav.safety": "安全",
    "nav.docs": "文档",
    "hero.kicker": "非官方 Amazon Photos 桌面客户端",
    "hero.title": "把照片<em>安静地</em>送进云端。",
    "hero.lede": "一次性上传、单向文件夹备份、传输队列、校验后的恢复下载。一个小巧的原生应用，而不是又一个相册。",
    "hero.source": "查看源码",
    "dl.generic": "下载",
    "dl.mac": "下载 macOS 版",
    "dl.windows": "下载 Windows 版",
    "dl.linux": "下载 Linux 版",
    "dl.all": "全部下载",
    "dl.note": "macOS · Windows · Linux — 免费开源",
    "dl.noteMac": "Apple 芯片 · <a href=\"{intel}\">Intel 版</a> · <a href=\"{all}\">其他平台</a>",
    "dl.noteOther": "{arch} · <a href=\"{all}\">其他平台</a>",
    "features.eyebrow": "四件事",
    "features.title": "只做四件事，<em>认真地</em>做。",
    "features.lede": "没有时间线，没有相册管理，没有 AI 搜索。只负责把文件安全地传上去，并在你需要时取回来。",
    "p1.title": "上传",
    "p1.body": "拖入文件或文件夹。在第一个字节离开硬盘之前，先看清数量、大小和会被跳过的文件。",
    "p2.title": "备份",
    "p2.body": "选一个文件夹。新增和改动的文件会立即或按间隔上传，子文件夹结构原样保留。",
    "p3.title": "传输",
    "p3.body": "持久化的队列，可暂停、继续、取消、重试。退出、崩溃、睡眠之后都还在。",
    "p4.title": "恢复",
    "p4.body": "按历史记录、文件夹或相册取回原图。每个文件落盘前都会校验大小和 MD5。",
    "oneway.eyebrow": "单向备份",
    "oneway.title": "水只往<em>一个方向</em>流。",
    "oneway.lede": "新照片向上走。在电脑上删掉的东西，永远不会从云端删除。试试看。",
    "oneway.local": "这台电脑",
    "oneway.cloud": "Amazon Photos",
    "oneway.add": "拍一张",
    "oneway.hint": "删掉一个本地文件，或者拍一张新照片。",
    "oneway.deleted": "已在本地删除 {name}，云端副本保留。",
    "oneway.uploaded": "{name} 已备份。",
    "oneway.full": "演示用的照片已经够多了。",
    "oneway.synced": "已备份",
    "oneway.kept": "仍在云端",
    "oneway.uploading": "上传中…",
    "oneway.delete": "从这台电脑删除 {name}",
    "queue.eyebrow": "传输队列",
    "queue.title": "中断了？先<em>核对</em>，再重传。",
    "queue.lede": "上传到一半应用退出，也不会盲目重复上传。下次启动时，Rainforest 按文件名和 MD5 核对哪些已经到达云端，确认它们，只重试真正没传上去的。",
    "q.idle": "{n} 个文件排队中",
    "q.running": "上传中 · 已确认 {done}/{n}",
    "q.quit": "应用在传输中途退出",
    "q.reconcile": "重新打开 · 正在核对云端",
    "q.done": "{n} 个文件全部确认",
    "q.start": "开始",
    "q.quitAction": "中途退出",
    "q.replay": "重放",
    "q.queued": "排队中",
    "q.unconfirmed": "未确认",
    "q.checking": "比对 MD5…",
    "q.verified": "MD5 一致 · 不重传",
    "q.retry": "云端没有 · 重试",
    "q.confirmed": "已确认",
    "details.eyebrow": "细节",
    "details.title": "由许多<em>小决定</em>组成。",
    "d1.t": "尊重外接硬盘",
    "d1.b": "拔掉备份源时，规则会以“源不可用”暂停，而不是把你的图库当成已删除。",
    "d2.t": "写到一半的文件会等待",
    "d2.b": "仍在复制或导出的文件，会等到大小稳定后再处理。",
    "d3.t": "文件夹保持原样",
    "d3.b": "本地子文件夹会镜像到目标位置下；相册则保持扁平。",
    "d4.t": "登录会话只留在本地",
    "d4.b": "通过 Amazon 自己的窗口登录。Cookie 存在权限受限的本地保险库里，所有日志都会脱敏。",
    "d5.t": "English 与简体中文",
    "d5.b": "跟随系统语言，也可以在设置或「显示」菜单里切换。",
    "d6.t": "每个桌面系统都是原生",
    "d6.b": "macOS、Windows、Linux，Intel 与 ARM。单个 Go 二进制，没有 Electron。",
    "close.title": "让雨<em>向上</em>落。",
    "close.lede": "免费，以 AGPL-3.0 开源，小到你会忘记它在运行。",
    "footer.disclaimer": "Rainforest 是非官方客户端，与 Amazon.com, Inc. 无关联，亦未获其认可或赞助。",
    "footer.changelog": "更新日志",
    "footer.security": "安全",
  },
};

const LANG_KEY = "rainforest.lang";
const storage = {
  get(key) { try { return localStorage.getItem(key); } catch { return null; } },
  set(key, value) { try { localStorage.setItem(key, value); } catch { /* private mode */ } },
};

let lang = storage.get(LANG_KEY) || (/^zh\b/i.test(navigator.language) ? "zh" : "en");
const langListeners = [];

function t(key, vars) {
  const s = STRINGS[lang][key] ?? STRINGS.en[key] ?? key;
  return vars ? s.replace(/\{(\w+)\}/g, (_, k) => vars[k] ?? "") : s;
}

function setText(el, key, vars) {
  el.dataset.i18n = key;
  if (vars) el.dataset.i18nVars = JSON.stringify(vars);
  else delete el.dataset.i18nVars;
  el.textContent = t(key, vars);
}

function applyLang() {
  document.documentElement.lang = lang === "zh" ? "zh-CN" : "en";
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    const vars = el.dataset.i18nVars ? JSON.parse(el.dataset.i18nVars) : undefined;
    el.textContent = t(el.dataset.i18n, vars);
  });
  document.querySelectorAll("[data-i18n-html]").forEach((el) => {
    el.innerHTML = t(el.dataset.i18nHtml);
  });
  langListeners.forEach((fn) => fn());
}

document.querySelector("[data-lang-toggle]").addEventListener("click", () => {
  lang = lang === "zh" ? "en" : "zh";
  storage.set(LANG_KEY, lang);
  applyLang();
});

/* ---------- nav + reveal ---------- */

const nav = document.querySelector(".nav");
const onScroll = () => nav.toggleAttribute("data-scrolled", scrollY > 8);
addEventListener("scroll", onScroll, { passive: true });
onScroll();

const revealer = new IntersectionObserver((entries) => {
  for (const entry of entries) {
    if (!entry.isIntersecting) continue;
    entry.target.setAttribute("data-shown", "");
    revealer.unobserve(entry.target);
  }
}, { rootMargin: "0px 0px -12% 0px" });
document.querySelectorAll(".reveal").forEach((el) => revealer.observe(el));

/* ---------- rain ---------- */

function rain(canvas) {
  const ctx = canvas.getContext("2d");
  const pointer = { x: -1e4, y: -1e4 };
  let w = 0, h = 0, dpr = 1, drops = [], motes = [], running = false, last = 0;

  const rand = (a, b) => a + Math.random() * (b - a);
  const drop = (y = rand(-h, h)) => ({ x: rand(-40, w + 40), y, len: rand(10, 26), v: rand(380, 720), a: rand(0.05, 0.16), ox: 0 });
  const mote = (y = rand(0, h)) => ({ x: rand(0, w), y, r: rand(0.8, 1.6), v: rand(18, 42), a: rand(0.25, 0.6), phase: rand(0, Math.PI * 2) });

  function resize() {
    const rect = canvas.getBoundingClientRect();
    dpr = Math.min(devicePixelRatio || 1, 2);
    w = rect.width;
    h = rect.height;
    canvas.width = Math.round(w * dpr);
    canvas.height = Math.round(h * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    const n = Math.round((w * h) / 7000);
    drops = Array.from({ length: n }, () => drop());
    motes = Array.from({ length: Math.max(6, Math.round(n / 14)) }, () => mote());
  }

  function draw(dt) {
    ctx.clearRect(0, 0, w, h);
    ctx.lineWidth = 1;
    ctx.lineCap = "round";
    const slant = 0.06;
    for (const d of drops) {
      d.y += d.v * dt;
      d.x += d.v * slant * dt;
      if (d.y - d.len > h) Object.assign(d, drop(rand(-60, -10)));
      const dx = d.x - pointer.x, dy = d.y - pointer.y;
      const dist = Math.hypot(dx, dy);
      const target = dist < 110 ? (dx / (dist || 1)) * (110 - dist) * 0.5 : 0;
      d.ox += (target - d.ox) * Math.min(1, dt * 10);
      const x = d.x + d.ox;
      ctx.strokeStyle = `rgba(206, 226, 214, ${d.a})`;
      ctx.beginPath();
      ctx.moveTo(x, d.y);
      ctx.lineTo(x - d.len * slant, d.y - d.len);
      ctx.stroke();
    }
    for (const m of motes) {
      m.y -= m.v * dt;
      m.phase += dt * 1.4;
      if (m.y < -10) Object.assign(m, mote(h + 10));
      const fade = Math.min(1, m.y / (h * 0.35), (h - m.y) / 80);
      ctx.fillStyle = `rgba(156, 199, 170, ${m.a * Math.max(0, fade)})`;
      ctx.beginPath();
      ctx.arc(m.x + Math.sin(m.phase) * 6, m.y, m.r, 0, Math.PI * 2);
      ctx.fill();
    }
  }

  function frame(now) {
    if (!running) return;
    draw(Math.min(0.05, (now - last) / 1000));
    last = now;
    requestAnimationFrame(frame);
  }

  function start() {
    if (running || reducedMotion.matches) return;
    running = true;
    last = performance.now();
    requestAnimationFrame(frame);
  }
  const stop = () => { running = false; };

  resize();
  draw(0.016);
  canvas.setAttribute("data-ready", "");

  let resizeTimer;
  addEventListener("resize", () => {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(() => { resize(); if (!running) draw(0.016); }, 120);
  });

  const hero = canvas.parentElement;
  if (matchMedia("(pointer: fine)").matches) {
    hero.addEventListener("pointermove", (e) => {
      const rect = canvas.getBoundingClientRect();
      pointer.x = e.clientX - rect.left;
      pointer.y = e.clientY - rect.top;
    });
    hero.addEventListener("pointerleave", () => { pointer.x = pointer.y = -1e4; });
  }

  new IntersectionObserver(([entry]) => (entry.isIntersecting ? start() : stop())).observe(canvas);
  reducedMotion.addEventListener("change", () => (reducedMotion.matches ? stop() : start()));
}

rain(document.querySelector(".rain"));

/* ---------- download ---------- */

async function detectPlatform() {
  const uaData = navigator.userAgentData;
  let os = "", arch = "";
  if (uaData) {
    os = uaData.platform.toLowerCase();
    try { arch = (await uaData.getHighEntropyValues(["architecture"])).architecture || ""; } catch { /* denied */ }
  }
  const ua = navigator.userAgent;
  if (!os) os = /Mac/.test(ua) ? "macos" : /Win/.test(ua) ? "windows" : /Linux/.test(ua) ? "linux" : "";
  if (/iPhone|iPad|Android/.test(ua) || navigator.maxTouchPoints > 1 && /Mac/.test(ua)) return null;
  if (os.includes("mac")) return { os: "mac", arch: arch === "x86" ? "amd64" : "arm64" };
  if (os.includes("win")) return { os: "windows", arch: arch === "arm" || /ARM64/i.test(ua) ? "arm64" : "amd64" };
  if (os.includes("linux")) return { os: "linux", arch: arch === "arm" || /aarch64|arm64/i.test(ua) ? "arm64" : "amd64" };
  return null;
}

const ASSET_PATTERNS = {
  mac: (a) => new RegExp(`^rainforest-darwin-${a}\\.tar\\.gz$`),
  windows: (a) => new RegExp(`Setup.*\\.${a}\\.exe$`),
  linux: (a) => new RegExp(`_${a}\\.deb$`),
};

async function setupDownloads() {
  const platform = await detectPlatform();
  let release = null;
  try {
    const res = await fetch(`https://api.github.com/repos/${REPO}/releases/latest`, { headers: { Accept: "application/vnd.github+json" } });
    if (res.ok) release = await res.json();
  } catch { /* offline: keep static links */ }

  if (release?.tag_name) document.querySelectorAll("[data-version]").forEach((el) => { el.textContent = release.tag_name; });
  if (!platform) return;

  const assets = release?.assets ?? [];
  const find = (os, arch) => assets.find((a) => ASSET_PATTERNS[os](arch).test(a.name))?.browser_download_url;
  const fallback = (os, arch) => (os === "mac" ? `${RELEASES}/download/rainforest-darwin-${arch}.tar.gz` : null);
  const url = find(platform.os, platform.arch) || fallback(platform.os, platform.arch);
  if (!url) return;

  document.querySelectorAll("[data-download]").forEach((a) => { a.href = url; });
  document.querySelectorAll("[data-download-label]").forEach((el) => { el.dataset.i18n = `dl.${platform.os}`; });

  const note = document.querySelector("[data-download-note]");
  const renderNote = () => {
    const vars = {
      all: RELEASES,
      intel: find("mac", "amd64") || fallback("mac", "amd64"),
      arch: platform.arch === "arm64" ? "ARM64" : "x64",
    };
    note.innerHTML = platform.os === "mac" && platform.arch === "arm64" ? t("dl.noteMac", vars) : t("dl.noteOther", vars);
  };
  delete note.dataset.i18n;
  langListeners.push(renderNote);
  applyLang();
}

/* ---------- one-way demo ---------- */

const THUMBS = [
  "linear-gradient(160deg, #2c4636, #a3c193)",
  "linear-gradient(160deg, #1d2b36, #7d9db3)",
  "linear-gradient(160deg, #4a3322, #d8a66d)",
  "linear-gradient(160deg, #23302a, #6f8f6a 60%, #c9d8a8)",
  "linear-gradient(160deg, #2f2a3a, #a491b8)",
  "linear-gradient(160deg, #3b4a2a, #d6cf8a)",
  "linear-gradient(160deg, #18302e, #5fa39a)",
];

function oneWay(root) {
  const local = root.querySelector("[data-local]");
  const cloud = root.querySelector("[data-cloud]");
  const addBtn = root.querySelector("[data-add]");
  const drop = root.querySelector("[data-drop]");
  const flow = root.querySelector(".flow");
  const status = root.querySelector("[data-oneway-status]");
  const MAX = 7;
  let serial = 2040;
  const queue = [];
  let busy = false;

  const nextName = () => `IMG_${++serial}.HEIC`;

  function row(name, thumb) {
    const li = document.createElement("li");
    li.className = "row";
    li.innerHTML = `<div class="row-inner"><div class="row-body"><span class="thumb" style="--thumb:${thumb}"></span><span class="file-name">${name}</span></div></div>`;
    return li;
  }

  function addLocal(name, thumb) {
    const li = row(name, thumb);
    const btn = document.createElement("button");
    btn.className = "icon-button";
    btn.type = "button";
    btn.innerHTML = svg(ICONS.trash);
    btn.setAttribute("aria-label", t("oneway.delete", { name }));
    langListeners.push(() => btn.setAttribute("aria-label", t("oneway.delete", { name })));
    btn.addEventListener("click", () => removeLocal(li, name));
    li.querySelector(".row-body").append(btn);
    local.append(li);
  }

  function addCloud(name, thumb, pending) {
    const li = row(name, thumb);
    li.dataset.name = name;
    const badge = document.createElement("span");
    badge.className = "badge";
    badge.innerHTML = `${svg(ICONS.cloudCheck)}<span class="badge-label"></span>`;
    li.querySelector(".row-body").append(badge);
    setBadge(li, pending ? "oneway.uploading" : "oneway.synced");
    li.toggleAttribute("data-pending", pending);
    cloud.append(li);
    return li;
  }

  function setBadge(li, key) {
    const badge = li.querySelector(".badge");
    const label = document.createElement("span");
    label.className = "badge-label";
    setText(label, key);
    badge.querySelector(".badge-label").replaceWith(label);
    badge.toggleAttribute("data-kept", key === "oneway.kept");
  }

  function removeLocal(li, name) {
    if (li.hasAttribute("data-gone")) return;
    const next = li.nextElementSibling || li.previousElementSibling;
    li.setAttribute("data-gone", "");
    setTimeout(() => li.remove(), 320);
    next?.querySelector("button")?.focus({ preventScroll: true });
    const twin = cloud.querySelector(`[data-name="${name}"]`);
    if (twin && !twin.hasAttribute("data-pending")) setBadge(twin, "oneway.kept");
    setText(status, "oneway.deleted", { name });
    status.dataset.tone = "ok";
    syncAdd();
  }

  function travel() {
    if (reducedMotion.matches) return Promise.resolve();
    const vertical = flow.clientHeight > flow.clientWidth;
    const distance = (vertical ? flow.clientHeight : flow.clientWidth) - 6;
    const axis = vertical ? "translateY" : "translateX";
    return drop.animate(
      [
        { transform: `${axis}(0)`, opacity: 0 },
        { opacity: 1, offset: 0.2 },
        { opacity: 1, offset: 0.8 },
        { transform: `${axis}(${distance}px)`, opacity: 0 },
      ],
      { duration: 900, easing: "cubic-bezier(0.77, 0, 0.175, 1)" },
    ).finished;
  }

  async function pump() {
    if (busy) return;
    busy = true;
    while (queue.length) {
      const { name, li } = queue.shift();
      await travel();
      await new Promise((r) => setTimeout(r, 120));
      li.removeAttribute("data-pending");
      setBadge(li, "oneway.synced");
      setText(status, "oneway.uploaded", { name });
      status.dataset.tone = "ok";
    }
    busy = false;
  }

  function syncAdd() {
    addBtn.disabled = cloud.children.length >= MAX;
  }

  addBtn.addEventListener("click", () => {
    const name = nextName();
    const thumb = THUMBS[serial % THUMBS.length];
    addLocal(name, thumb);
    queue.push({ name, li: addCloud(name, thumb, true) });
    syncAdd();
    if (addBtn.disabled) { setText(status, "oneway.full"); status.dataset.tone = ""; }
    pump();
  });

  for (let i = 0; i < 4; i++) {
    const name = nextName();
    const thumb = THUMBS[serial % THUMBS.length];
    addLocal(name, thumb);
    addCloud(name, thumb, false);
  }
}

/* ---------- queue demo ---------- */

const QUEUE_FILES = [
  { name: "IMG_2041.HEIC", mb: 4.2 },
  { name: "MVI_0310.MOV", mb: 182 },
  { name: "DSC_7781.ARW", mb: 48.6 },
  { name: "IMG_2042.HEIC", mb: 3.8 },
  { name: "IMG_2043.JPG", mb: 6.1 },
];

const STATE_ICON = { queued: "dashed", running: "notch", unconfirmed: "warning", checking: "notch", confirmed: "check", verified: "check" };

function transferQueue(root) {
  const list = root.querySelector("[data-queue-list]");
  const stateEl = root.querySelector("[data-queue-state]");
  const action = root.querySelector("[data-queue-action]");
  const CONCURRENCY = 2;
  let items = [], phase = "idle", visible = false, last = 0, raf = 0, timers = [], started = false;

  const later = (fn, ms) => timers.push(setTimeout(fn, ms));

  function build() {
    list.textContent = "";
    items = QUEUE_FILES.map((f) => {
      const li = document.createElement("li");
      li.className = "qrow";
      li.innerHTML = `<span class="qicon">${svg(ICONS.dashed)}${svg(ICONS.notch, "spin")}${svg(ICONS.warning)}${svg(ICONS.check)}</span><span class="file-name">${f.name}</span><span class="qmeta mono"></span><span class="qbar"></span>`;
      list.append(li);
      return { ...f, li, p: 0, state: "", duration: 1400 + f.mb * 14, meta: li.querySelector(".qmeta"), bar: li.querySelector(".qbar") };
    });
    items.forEach((it) => setState(it, "queued"));
  }

  function setState(it, state) {
    it.state = state;
    it.li.dataset.state = state;
    const icons = it.li.querySelectorAll(".qicon svg");
    const index = ["dashed", "notch", "warning", "check"].indexOf(STATE_ICON[state]);
    icons.forEach((el, i) => el.toggleAttribute("data-show", i === index));
    if (state === "running") renderProgress(it);
    else setText(it.meta, `q.${state}`);
    if (state === "queued") { it.p = 0; it.bar.style.transform = "scaleX(0)"; }
  }

  function renderProgress(it) {
    it.bar.style.transform = `scaleX(${it.p})`;
    delete it.meta.dataset.i18n;
    it.meta.textContent = `${(it.mb * it.p).toFixed(1)} / ${it.mb} MB`;
  }

  const done = () => items.filter((it) => it.state === "confirmed" || it.state === "verified").length;

  function render() {
    root.dataset.phase = phase;
    const vars = { n: items.length, done: done() };
    setText(stateEl, `q.${phase}`, vars);
    const label = { idle: "q.start", running: "q.quitAction", done: "q.replay" }[phase] || "q.quitAction";
    setText(action, label);
    action.disabled = phase === "quit" || phase === "reconcile";
  }

  function tick(now) {
    const dt = Math.min(64, now - last);
    last = now;
    if (phase !== "running" || !visible) return;
    for (const it of items.filter((x) => x.state === "running")) {
      it.p = Math.min(1, it.p + dt / it.duration);
      renderProgress(it);
      if (it.p >= 1) { setState(it, "confirmed"); render(); }
    }
    const running = items.filter((x) => x.state === "running").length;
    items.filter((x) => x.state === "queued").slice(0, CONCURRENCY - running).forEach((it) => setState(it, "running"));
    if (done() === items.length) { phase = "done"; render(); return; }
    raf = requestAnimationFrame(tick);
  }

  function run() {
    phase = "running";
    render();
    last = performance.now();
    cancelAnimationFrame(raf);
    raf = requestAnimationFrame(tick);
  }

  function quit() {
    phase = "quit";
    cancelAnimationFrame(raf);
    const interrupted = items.filter((x) => x.state === "running").sort((a, b) => b.p - a.p);
    interrupted.forEach((it) => setState(it, "unconfirmed"));
    render();
    later(() => reconcile(interrupted), 1200);
  }

  function reconcile(interrupted) {
    phase = "reconcile";
    render();
    interrupted.forEach((it, i) => {
      later(() => setState(it, "checking"), 300 + i * 500);
      later(() => {
        const arrived = interrupted.length > 1 ? i === 0 : it.p > 0.6;
        if (arrived) { it.p = 1; it.bar.style.transform = "scaleX(1)"; setState(it, "verified"); }
        else { it.meta.dataset.i18n = "q.retry"; setText(it.meta, "q.retry"); it.li.dataset.state = "unconfirmed"; }
        render();
      }, 1300 + i * 500);
    });
    later(() => {
      interrupted.filter((it) => it.state !== "verified").forEach((it) => setState(it, "queued"));
      run();
    }, 2300 + interrupted.length * 500);
  }

  action.addEventListener("click", () => {
    if (phase === "idle") run();
    else if (phase === "running") quit();
    else if (phase === "done") { timers.forEach(clearTimeout); timers = []; build(); run(); }
  });

  new IntersectionObserver(([entry]) => {
    visible = entry.isIntersecting;
    if (!visible) return;
    if (phase === "running") run();
    else if (phase === "idle" && !started) { started = true; later(() => phase === "idle" && run(), 500); }
  }, { threshold: 0.5 }).observe(root);

  build();
  render();
}

/* ---------- ambient pointer: dappled light + window tilt ---------- */

function ambient() {
  if (!matchMedia("(hover: hover) and (pointer: fine)").matches) return;

  document.querySelectorAll(".demo").forEach((demo) => {
    const glow = document.createElement("span");
    glow.className = "glow";
    demo.prepend(glow);
    demo.addEventListener("pointerenter", () => demo.setAttribute("data-lit", ""));
    demo.addEventListener("pointerleave", () => demo.removeAttribute("data-lit"));
    demo.addEventListener("pointermove", (e) => {
      const r = demo.getBoundingClientRect();
      glow.style.transform = `translate3d(${e.clientX - r.left}px, ${e.clientY - r.top}px, 0)`;
    });
  });

  const shot = document.querySelector(".shot");
  const tilt = shot.querySelector(".shot-tilt");
  const s = { x: 0, y: 0, vx: 0, vy: 0, tx: 0, ty: 0 };
  let raf = 0, last = 0;

  function step(now) {
    const dt = Math.min(1 / 30, (now - last) / 1000);
    last = now;
    for (const [p, v, t] of [["x", "vx", "tx"], ["y", "vy", "ty"]]) {
      s[v] += (-90 * (s[p] - s[t]) - 14 * s[v]) * dt;
      s[p] += s[v] * dt;
    }
    tilt.style.transform = `perspective(1600px) rotateX(${(-s.y * 2.4).toFixed(3)}deg) rotateY(${(s.x * 3.2).toFixed(3)}deg)`;
    const settled = Math.abs(s.x - s.tx) + Math.abs(s.y - s.ty) + Math.abs(s.vx) + Math.abs(s.vy) < 0.001;
    raf = settled ? 0 : requestAnimationFrame(step);
  }
  const kick = () => { if (!raf && !reducedMotion.matches) { last = performance.now(); raf = requestAnimationFrame(step); } };

  addEventListener("pointermove", (e) => {
    const r = shot.getBoundingClientRect();
    if (r.bottom < 0 || r.top > innerHeight) return;
    s.tx = clampUnit((e.clientX - (r.left + r.width / 2)) / (r.width / 2));
    s.ty = clampUnit((e.clientY - (r.top + r.height / 2)) / (r.height / 2));
    kick();
  }, { passive: true });
  document.addEventListener("pointerleave", () => { s.tx = s.ty = 0; kick(); });
}

const clampUnit = (v) => Math.max(-1, Math.min(1, v));

ambient();
oneWay(document.querySelector("[data-oneway]"));
transferQueue(document.querySelector("[data-queue]"));
applyLang();
setupDownloads();
