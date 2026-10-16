"use strict";

(() => {
  const reduce = matchMedia("(prefers-reduced-motion: reduce)");
  const finePointer = matchMedia("(pointer: fine)");
  const NS = "http://www.w3.org/2000/svg";
  const INTRO_KEY = "rainforest.intro";
  let uid = 0;

  const rad = (deg) => (deg * Math.PI) / 180;
  const clamp = (v, a, b) => Math.min(b, Math.max(a, v));
  const lerp = (a, b, t) => a + (b - a) * t;

  function seeded(seed) {
    return () => {
      seed |= 0; seed = (seed + 0x6d2b79f5) | 0;
      let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
      t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }

  /* Samples a damped spring into a CSS linear() easing so WAAPI can run it off the main thread. */
  function springEasing(bounce) {
    const zeta = 1 - bounce, omega = 2 * Math.PI, wd = omega * Math.sqrt(1 - zeta * zeta);
    const x = (t) => 1 - Math.exp(-zeta * omega * t) * (Math.cos(wd * t) + ((zeta * omega) / wd) * Math.sin(wd * t));
    let end = 0.5;
    while (end < 3 && Math.abs(1 - x(end)) + Math.abs(1 - x(end + 0.05)) > 0.002) end += 0.05;
    const points = Array.from({ length: 48 }, (_, i) => +x((i / 47) * end).toFixed(4));
    points[points.length - 1] = 1;
    const supported = CSS.supports("animation-timing-function", "linear(0, 1)");
    return { easing: supported ? `linear(${points.join(", ")})` : "cubic-bezier(0.23, 1, 0.32, 1)", length: supported ? end : 1 };
  }
  const SPRING = springEasing(0.2);

  /* ---------- leaf shapes: base at (0,0), growing toward -y ---------- */

  const pt = (x, y) => `${x.toFixed(1)},${y.toFixed(1)}`;

  function frond(L, { curve = 0.4, density = 24, reach = 0.34, width = 0.075, spread = [72, 28] }) {
    const c0 = { x: 0, y: 0 }, c1 = { x: curve * L * 0.32, y: -L * 0.55 }, c2 = { x: curve * L * 0.62, y: -L * 0.96 };
    const at = (t) => ({
      x: (1 - t) ** 2 * c0.x + 2 * (1 - t) * t * c1.x + t * t * c2.x,
      y: (1 - t) ** 2 * c0.y + 2 * (1 - t) * t * c1.y + t * t * c2.y,
    });
    const tangent = (t) => {
      const dx = 2 * (1 - t) * (c1.x - c0.x) + 2 * t * (c2.x - c1.x);
      const dy = 2 * (1 - t) * (c1.y - c0.y) + 2 * t * (c2.y - c1.y);
      const m = Math.hypot(dx, dy);
      return { x: dx / m, y: dy / m };
    };
    let blades = "";
    for (let i = 0; i < density; i++) {
      const t = 0.1 + (i / (density - 1)) * 0.88;
      const b = at(t), tg = tangent(t);
      const len = L * reach * Math.sin(Math.PI * Math.min(1, t * 1.05)) ** 0.6 * (1 - 0.35 * t) + L * 0.03;
      for (const side of [-1, 1]) {
        const ang = rad(side * lerp(spread[0], spread[1], t));
        const dir = { x: tg.x * Math.cos(ang) - tg.y * Math.sin(ang), y: tg.x * Math.sin(ang) + tg.y * Math.cos(ang) };
        const tip = { x: b.x + dir.x * len - tg.x * len * 0.12, y: b.y + dir.y * len - tg.y * len * 0.12 };
        const n = { x: -dir.y, y: dir.x }, w = len * width;
        const m = { x: b.x + dir.x * len * 0.45, y: b.y + dir.y * len * 0.45 };
        blades += `M${pt(b.x, b.y)}Q${pt(m.x + n.x * w, m.y + n.y * w)} ${pt(tip.x, tip.y)}Q${pt(m.x - n.x * w * 0.6, m.y - n.y * w * 0.6)} ${pt(b.x, b.y)}Z`;
      }
    }
    return { body: blades, rib: `M0,0Q${pt(c1.x, c1.y)} ${pt(c2.x, c2.y)}`, ribWidth: L * 0.012, length: L };
  }

  function monstera(L, { slits = 5, stem = 0.38, holes = true }) {
    const W = L * 0.86, S = L * stem, y = (f) => -S - L * f;
    const body =
      `M0,${y(0.06)}C${pt(W * 0.14, y(-0.1))} ${pt(W * 0.44, y(-0.08))} ${pt(W * 0.5, y(0.26))}` +
      `C${pt(W * 0.56, y(0.6))} ${pt(W * 0.3, y(0.93))} ${pt(W * 0.02, y(1))}` +
      `C${pt(-W * 0.3, y(0.92))} ${pt(-W * 0.57, y(0.6))} ${pt(-W * 0.5, y(0.26))}` +
      `C${pt(-W * 0.44, y(-0.08))} ${pt(-W * 0.14, y(-0.1))} 0,${y(0.06)}Z`;
    let cuts = "", wedges = "", veins = "";
    for (let i = 0; i < slits; i++) {
      const f = 0.14 + (i / slits) * 0.72;
      for (const side of [-1, 1]) {
        const a = rad(side * lerp(78, 52, i / slits));
        const sx = Math.sin(a), sy = -Math.cos(a);
        const start = L * (0.13 + 0.03 * Math.sin(i * 2.3 + side));
        const half = L * 0.02, nx = -sy * half, ny = sx * half;
        wedges += `M${pt(sx * start, y(f) + sy * start)}L${pt(sx * L + nx, y(f) + sy * L + ny)}L${pt(sx * L - nx, y(f) + sy * L - ny)}Z`;
        veins += `M0,${y(f - 0.03)}L${pt(sx * L * 0.12, y(f) + sy * L * 0.12)}`;
        if (holes && i > 0 && i < slits - 1) {
          const hf = f - 0.07, hx = side * L * 0.075;
          cuts += `M${pt(hx, y(hf) - L * 0.022)}l0,${(L * 0.044).toFixed(1)}`;
        }
      }
    }
    return { body, cuts, wedges, cutWidth: L * 0.026, veins, rib: `M0,0L0,${y(0.92)}`, ribWidth: L * 0.016, length: L + S };
  }

  function banana(L, { tears = 4 }) {
    const W = L * 0.3, S = L * 0.14, y = (f) => -S - L * f;
    const body = `M0,${-S}C${pt(W * 0.75, y(0.08))} ${pt(W * 0.62, y(0.8))} 0,${y(1)}C${pt(-W * 0.62, y(0.8))} ${pt(-W * 0.75, y(0.08))} 0,${-S}Z`;
    let cuts = "";
    for (let i = 0; i < tears; i++) {
      const f = 0.22 + i * (0.6 / tears) + 0.05 * Math.sin(i * 3.1);
      const side = i % 2 ? 1 : -1;
      cuts += `M${pt(side * W * 0.1, y(f))}L${pt(side * W, y(f + 0.09))}`;
    }
    let veins = "";
    for (let i = 1; i < 9; i++) veins += `M0,${y(i * 0.1)}L${pt(W * 0.42, y(i * 0.1 + 0.06))}M0,${y(i * 0.1)}L${pt(-W * 0.42, y(i * 0.1 + 0.06))}`;
    return { body, cuts, cutWidth: L * 0.012, veins, rib: `M0,0L0,${y(0.96)}`, ribWidth: L * 0.014, length: L + S };
  }

  const SHAPES = {
    palm: (L, o) => frond(L, o),
    fern: (L, o) => frond(L, { density: 30, reach: 0.2, width: 0.16, spread: [64, 40], ...o }),
    monstera: (L, o) => monstera(L, o),
    heart: (L, o) => monstera(L, { slits: 0, stem: 0.5, ...o }),
    banana: (L, o) => banana(L, o),
  };

  const DEPTH_TONES = [
    { base: "#0b140f", tip: "#16271d", vein: "rgba(170, 210, 182, 0.05)" },
    { base: "#0a140e", tip: "#1a3123", vein: "rgba(170, 210, 182, 0.08)" },
    { base: "#08100b", tip: "#1a3324", vein: "rgba(186, 222, 196, 0.12)" },
  ];

  function leafSVG(spec, L, flip) {
    const shape = SHAPES[spec.type](L, { ...spec.shape, curve: (spec.shape?.curve ?? 0.4) * (flip ? -1 : 1) });
    const tone = DEPTH_TONES[spec.depth];
    const reachX = shape.length * 0.75, top = shape.length * 1.06, bottom = shape.length * 0.2;
    const id = `leaf${++uid}`;
    const svg = document.createElementNS(NS, "svg");
    svg.setAttribute("viewBox", `${-reachX} ${-top} ${reachX * 2} ${top + bottom}`);
    svg.setAttribute("width", reachX * 2);
    svg.setAttribute("height", top + bottom);
    svg.style.left = `${-reachX}px`;
    svg.style.top = `${-top}px`;
    svg.style.transformOrigin = `${reachX}px ${top}px`;
    const mask = shape.cuts || shape.wedges
      ? `<mask id="${id}m" maskUnits="userSpaceOnUse" x="${-reachX}" y="${-top}" width="${reachX * 2}" height="${top + bottom}"><rect x="${-reachX}" y="${-top}" width="${reachX * 2}" height="${top + bottom}" fill="#fff"/><path d="${shape.cuts || ""}" stroke="#000" stroke-width="${shape.cutWidth}" stroke-linecap="round" fill="none"/><path d="${shape.wedges || ""}" fill="#000"/></mask>`
      : "";
    const clip = shape.veins ? `<clipPath id="${id}c"><path d="${shape.body}"/></clipPath>` : "";
    svg.innerHTML =
      `<defs><linearGradient id="${id}g" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="${flip ? -shape.length * 0.2 : shape.length * 0.2}" y2="${-shape.length}"><stop offset="0" stop-color="${tone.base}"/><stop offset="1" stop-color="${tone.tip}"/></linearGradient>${mask}${clip}</defs>` +
      `<g${mask ? ` mask="url(#${id}m)"` : ""}>` +
      `<path d="${shape.rib}" stroke="${tone.base}" stroke-width="${shape.ribWidth * 1.4}" stroke-linecap="round" fill="none"/>` +
      `<path d="${shape.body}" fill="url(#${id}g)"/>` +
      (shape.veins ? `<path d="${shape.veins}" clip-path="url(#${id}c)" stroke="${tone.vein}" stroke-width="${shape.ribWidth * 0.45}" fill="none"/>` : "") +
      `<path d="${shape.rib}" stroke="${tone.vein}" stroke-width="${shape.ribWidth * 0.6}" stroke-linecap="round" fill="none"/></g>`;
    return { svg, length: shape.length };
  }

  function fit(svg, b) {
    const x = b.x - 4, y = b.y - 4, w = b.width + 8, h = b.height + 8;
    svg.setAttribute("viewBox", `${x} ${y} ${w} ${h}`);
    svg.setAttribute("width", w);
    svg.setAttribute("height", h);
    svg.style.left = `${x}px`;
    svg.style.top = `${y}px`;
    svg.style.transformOrigin = `${-x}px ${-y}px`;
  }

  /* ---------- layouts: x/y are fractions of the canopy box, L in canopy units, rest angle 0 = up, + = toward center ---------- */

  const HERO = [
    { type: "palm", depth: 0, x: -0.02, y: 1.02, L: 60, rest: 24, shape: { curve: 0.5 } },
    { type: "palm", depth: 0, x: 0.07, y: 1.05, L: 44, rest: 52, shape: { curve: 0.6 }, wide: true },
    { type: "banana", depth: 0, x: -0.05, y: 0.88, L: 40, rest: 58 },
    { type: "palm", depth: 0, x: -0.05, y: -0.06, L: 40, rest: 140, shape: { curve: -0.4 } },
    { type: "banana", depth: 1, x: -0.02, y: 1.04, L: 50, rest: 10 },
    { type: "monstera", depth: 1, x: 0.03, y: 1.08, L: 28, rest: 34 },
    { type: "fern", depth: 1, x: 0.11, y: 1.06, L: 32, rest: 64, shape: { curve: 0.7 }, wide: true },
    { type: "monstera", depth: 1, x: -0.12, y: 0.62, L: 30, rest: -24, intro: 76, curtain: true },
    { type: "fern", depth: 1, x: -0.03, y: -0.04, L: 30, rest: 152, shape: { curve: -0.6 }, wide: true },
    { type: "monstera", depth: 2, x: -0.05, y: 1.12, L: 38, rest: 46 },
    { type: "palm", depth: 2, x: -0.08, y: 0.96, L: 46, rest: 70, shape: { curve: 0.35 } },
    { type: "heart", depth: 2, x: 0.13, y: 1.14, L: 20, rest: 18, wide: true },
    { type: "monstera", depth: 2, x: -0.16, y: 0.42, L: 56, rest: -44, intro: 84, curtain: true },
    { type: "banana", depth: 2, x: -0.08, y: 0.18, L: 62, rest: 168, intro: 104, curtain: true },
    { type: "banana", depth: 1, x: -0.12, y: 0.74, L: 60, rest: -52, intro: 90, curtain: true },
    { type: "palm", depth: 2, x: -0.12, y: 0.32, L: 56, rest: -64, intro: 116, curtain: true, shape: { curve: 0.3 } },
  ];

  const CLOSING = [
    { type: "palm", depth: 0, x: -0.02, y: 1.04, L: 54, rest: 30, shape: { curve: 0.5 } },
    { type: "banana", depth: 1, x: 0.0, y: 1.06, L: 44, rest: 14 },
    { type: "monstera", depth: 2, x: -0.03, y: 1.1, L: 30, rest: 50 },
    { type: "fern", depth: 1, x: 0.08, y: 1.06, L: 28, rest: 66, shape: { curve: 0.7 }, wide: true },
  ];

  /* ---------- shared pointer, scroll and ticker ---------- */

  const pointer = { x: -1e4, y: -1e4, vx: 0, vy: 0, at: 0 };
  addEventListener("pointermove", (e) => {
    const now = performance.now(), dt = Math.max(8, now - pointer.at);
    if (now - pointer.at < 100) {
      pointer.vx = lerp(pointer.vx, (e.clientX - pointer.x) / dt, 0.5);
      pointer.vy = lerp(pointer.vy, (e.clientY - pointer.y) / dt, 0.5);
    } else pointer.vx = pointer.vy = 0;
    pointer.x = e.clientX; pointer.y = e.clientY; pointer.at = now;
  }, { passive: true });
  document.addEventListener("pointerleave", () => { pointer.x = pointer.y = -1e4; });

  const canopies = [];
  let lastScroll = scrollY, running = false, last = 0;

  addEventListener("scroll", () => {
    const dy = scrollY - lastScroll;
    lastScroll = scrollY;
    if (reduce.matches) return;
    for (const c of canopies) if (c.visible) c.gust(dy);
    wake();
  }, { passive: true });

  function frame(now) {
    const dt = Math.min(1 / 30, (now - last) / 1000);
    last = now;
    const t = now / 1000;
    const live = canopies.filter((c) => c.visible);
    live.forEach((c) => c.step(t, dt));
    running = live.length > 0 && !reduce.matches && !document.hidden;
    if (running) requestAnimationFrame(frame);
  }
  function wake() {
    if (running || reduce.matches || document.hidden) return;
    running = true;
    last = performance.now();
    requestAnimationFrame(frame);
  }
  document.addEventListener("visibilitychange", wake);
  reduce.addEventListener("change", wake);

  /* ---------- canopy ---------- */

  function canopy(root, specs, { intro = "none", parallax = false, seed = 7, bleed = 0 } = {}) {
    const layers = [];
    let leaves = [], W = 0, H = 0, introDone = intro === "none";

    for (const side of [-1, 1]) {
      for (const depth of [0, 1, 2]) {
        const layer = document.createElement("div");
        layer.className = "canopy-layer";
        layer.dataset.depth = depth;
        root.append(layer);
        layers.push({ el: layer, side, depth });
      }
    }

    function build() {
      const rect = root.getBoundingClientRect();
      W = rect.width; H = rect.height - bleed;
      const mobile = W < 640;
      const unit = mobile ? Math.max(4.4, W / 90) : Math.min(W * 0.62, H) / 100;
      layers.forEach((l) => { l.el.textContent = ""; });
      const rand = seeded(seed);
      leaves = [];
      for (const side of [-1, 1]) {
        for (const spec of specs) {
          if (mobile && spec.wide) continue;
          const jitter = { L: lerp(0.88, 1.08, rand()), a: lerp(-7, 7, rand()) };
          const L = spec.L * unit * jitter.L;
          const flip = side === 1;
          const { svg, length } = leafSVG(spec, L, flip);
          const wrap = document.createElement("div");
          wrap.className = "leaf";
          wrap.append(svg);
          const fx = side === -1 ? spec.x : 1 - spec.x;
          const ax = fx * W, ay = spec.y * H;
          wrap.style.left = `${ax}px`;
          wrap.style.top = `${ay}px`;
          const sideSign = side === -1 ? 1 : -1;
          const rest = (spec.rest + jitter.a) * sideSign;
          const toCenter = (Math.atan2(W * 0.5 - ax, ay - H * 0.42) * 180) / Math.PI;
          const wobble = lerp(-12, 12, rand());
          const introAngle = (spec.intro != null ? spec.intro * sideSign : rest + clamp(toCenter - rest, -46, 46) * 0.9) + wobble;
          const layer = layers.find((l) => l.side === side && l.depth === spec.depth);
          layer.el.append(wrap);
          leaves.push({
            wrap, svg, side, depth: spec.depth, ax, ay, rest, introAngle, curtain: !!spec.curtain, shift: lerp(0.75, 1.25, rand()),
            reach: length * 0.55,
            size: clamp(length / (unit * 40), 0.6, 1.8),
            amp: lerp(0.7, 1.6, rand()) * (spec.depth === 2 ? 1 : 0.8),
            freq: lerp(0.35, 0.7, rand()),
            phase: rand() * Math.PI * 2,
            a: 0, v: 0, rendered: Infinity,
          });
        }
      }
      const boxes = leaves.map((l) => l.svg.querySelector("g").getBBox());
      leaves.forEach((l, i) => {
        fit(l.svg, boxes[i]);
        l.wrap.style.transform = `rotate(${l.rest}deg)`;
      });
    }

    function play() {
      const order = (l) => (2 - l.depth) * 1 + (l.curtain ? -1 : 0);
      const pending = leaves.map((l) => {
        if (intro === "part") {
          const push = (1 - l.depth * 0.25) * W * 0.12 * l.shift * (l.side === -1 ? 1 : -1);
          const from = `translate(${push.toFixed(0)}px, ${(-H * 0.05 * l.shift).toFixed(0)}px) rotate(${l.introAngle}deg) scale(${(1.1 + 0.15 * l.shift).toFixed(2)})`;
          const delay = 380 + order(l) * 110 + Math.random() * 90;
          return l.wrap.animate(
            [{ transform: from }, { transform: `rotate(${l.rest}deg)` }],
            { duration: 1100 * SPRING.length, delay, easing: SPRING.easing, fill: "backwards" },
          ).finished;
        }
        const delay = order(l) * 70 + Math.random() * 60;
        l.wrap.animate([{ opacity: 0 }, { opacity: 1 }], { duration: 500, delay, easing: "ease-out", fill: "backwards" });
        return l.wrap.animate(
          [{ transform: `rotate(${l.rest - l.side * -18}deg) scale(0.9)` }, { transform: `rotate(${l.rest}deg)` }],
          { duration: 900 * SPRING.length, delay, easing: SPRING.easing, fill: "backwards" },
        ).finished;
      });
      Promise.allSettled(pending).then(() => { introDone = true; wake(); });
    }

    const state = {
      visible: false,
      gust(dy) {
        const kick = clamp(dy, -60, 60) * 0.8;
        for (const l of leaves) l.v += kick * l.side * (0.6 + l.depth * 0.25) / l.size;
      },
      step(t, dt) {
        const rect = root.getBoundingClientRect();
        if (parallax) {
          const s = clamp(-rect.top, 0, H);
          for (const l of layers) {
            const k = [0.1, 0.18, 0.3][l.depth];
            l.el.style.transform = `translate3d(${(l.side * s * k * 0.5).toFixed(1)}px, ${(s * (l.depth === 0 ? 0.12 : -k * 0.4)).toFixed(1)}px, 0)`;
          }
        }
        if (!introDone) return;
        const gust = 0.55 + 0.3 * Math.sin(t * 0.21) + 0.15 * Math.sin(t * 0.83 + 1.7);
        const active = performance.now() - pointer.at < 1500;
        for (const l of leaves) {
          let target = l.amp * Math.sin(t * l.freq * Math.PI * 2 * 0.5 + l.phase) * gust;
          if (active) {
            const ang = rad(l.rest + l.a);
            const dir = { x: Math.sin(ang), y: -Math.cos(ang) };
            const ax = rect.left + l.ax, ay = rect.top + l.ay;
            const fx = ax + dir.x * l.reach, fy = ay + dir.y * l.reach;
            const R = l.reach * 0.9 + 60;
            const dist = Math.hypot(pointer.x - fx, pointer.y - fy);
            if (dist < R) {
              const fall = 1 - (dist / R) ** 2;
              const cross = dir.x * (pointer.y - ay) - dir.y * (pointer.x - ax);
              target -= Math.sign(cross) * fall * 11 / Math.sqrt(l.size);
              const swipe = dir.x * pointer.vy - dir.y * pointer.vx;
              l.v += clamp(swipe, -4, 4) * fall * 520 / l.size * dt;
            }
          }
          const k = 46 / l.size, c = 5.2;
          l.v += (-k * (l.a - target) - c * l.v) * dt;
          l.a = clamp(l.a + l.v * dt, -16, 16);
          if (Math.abs(l.a - l.rendered) > 0.02) {
            l.svg.style.transform = `rotate(${l.a.toFixed(2)}deg)`;
            l.rendered = l.a;
          }
        }
        pointer.vx *= 0.9; pointer.vy *= 0.9;
      },
    };

    build();
    canopies.push(state);

    let resizeTimer, width = innerWidth;
    addEventListener("resize", () => {
      if (innerWidth === width) return;
      width = innerWidth;
      clearTimeout(resizeTimer);
      resizeTimer = setTimeout(() => { leaves.forEach((l) => l.wrap.getAnimations().forEach((a) => a.finish())); build(); introDone = true; }, 150);
    });

    let played = false;
    const start = () => {
      played = true;
      requestAnimationFrame(() => root.setAttribute("data-ready", ""));
      if (reduce.matches) introDone = true;
      else play();
    };
    if (intro !== "unfurl") start();
    new IntersectionObserver(([entry]) => {
      state.visible = entry.isIntersecting;
      if (state.visible && !played) start();
      wake();
    }, { threshold: intro === "unfurl" ? 0.35 : 0 }).observe(root);
    return state;
  }

  const hero = document.querySelector("[data-canopy='hero']");
  const firstVisit = document.documentElement.hasAttribute("data-intro");
  if (hero) canopy(hero, HERO, { intro: firstVisit ? "part" : "settle", parallax: true, seed: 11, bleed: 160 });
  const closing = document.querySelector("[data-canopy='closing']");
  if (closing) canopy(closing, CLOSING, { intro: "unfurl", seed: 23 });
  try { sessionStorage.setItem(INTRO_KEY, "1"); } catch { /* storage blocked */ }
})();
