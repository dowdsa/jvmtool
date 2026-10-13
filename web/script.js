/* 复制安装命令 */
document.querySelectorAll(".copy-btn").forEach(function (btn) {
  btn.addEventListener("click", function () {
    var code = document.getElementById(btn.dataset.copy);
    if (!code) return;
    var text = code.textContent;
    var done = function () {
      btn.textContent = "已复制";
      btn.classList.add("done");
      setTimeout(function () {
        btn.textContent = "复制";
        btn.classList.remove("done");
      }, 1600);
    };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done).catch(function () {
        fallbackCopy(text, done);
      });
    } else {
      fallbackCopy(text, done);
    }
  });
});

function fallbackCopy(text, done) {
  var ta = document.createElement("textarea");
  ta.value = text;
  ta.style.position = "fixed";
  ta.style.opacity = "0";
  document.body.appendChild(ta);
  ta.select();
  try { document.execCommand("copy"); done(); } catch (e) { /* noop */ }
  document.body.removeChild(ta);
}

/* 移动端菜单 */
var toggle = document.querySelector(".nav-toggle");
var nav = document.querySelector(".site-nav");
if (toggle && nav) {
  toggle.addEventListener("click", function () {
    var open = nav.classList.toggle("open");
    toggle.setAttribute("aria-expanded", open ? "true" : "false");
  });
  nav.addEventListener("click", function (e) {
    if (e.target.tagName === "A") nav.classList.remove("open");
  });
}

var reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

/* 滚动渐入 */
if (!reducedMotion && "IntersectionObserver" in window) {
  var io = new IntersectionObserver(function (entries) {
    entries.forEach(function (en) {
      if (en.isIntersecting) {
        en.target.classList.add("shown");
        io.unobserve(en.target);
      }
    });
  }, { threshold: 0.15 });
  document.querySelectorAll("[data-reveal]").forEach(function (el) { io.observe(el); });
} else {
  document.querySelectorAll("[data-reveal]").forEach(function (el) { el.classList.add("shown"); });
}

/* 终端打字机动效 */
var termLines = [
  { cmd: "jm jdk search 17 --distro zulu", note: "# 在 Zulu 中搜索 JDK 17" },
  { cmd: "jm jdk install 17.0.20", note: "# 下载并安装（镜像优先）" },
  { cmd: "jm jdk use 17", note: "# 切换当前版本" },
  { cmd: "jm maven install 3.9.11 && jm maven use 3.9", note: "" },
  { cmd: "jm status", note: "# 查看已安装与当前版本" },
  { cmd: "jm jdk import /usr/lib/jvm/java-21", note: "# 收编系统里已有的 JDK" },
  { cmd: "jm doctor", note: "# 环境与代理体检" }
];

function esc(s) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function renderLine(line) {
  var note = line.note ? '      <span class="o">' + esc(line.note) + "</span>" : "";
  return '<span class="p">$ </span>' + esc(line.cmd) + note;
}

function startTyping(term) {
  var code = document.getElementById("term-code");
  if (!code) return;
  var li = 0, ci = 0, out = "";
  function pad(line, n) {
    return { cmd: line.cmd.slice(0, n), note: n >= line.cmd.length ? line.note : "" };
  }
  function step() {
    if (li >= termLines.length) {
      code.innerHTML = out + '<span class="cursor"></span>';
      return;
    }
    var line = termLines[li];
    if (ci <= line.cmd.length) {
      code.innerHTML = out + renderLine(pad(line, ci)) + '<span class="cursor"></span>';
      ci++;
      setTimeout(step, 22 + Math.random() * 26);
    } else {
      out += renderLine(line) + "\n";
      li++; ci = 0;
      setTimeout(step, 260);
    }
  }
  code.innerHTML = '<span class="cursor"></span>';
  setTimeout(step, 400);
}

/* 平台/架构检测 → 站内直接下载对应安装包 */
var REL_BASE = "https://github.com/dowdsa/jvmtool/releases/latest/download/";
var OS_LABEL = { windows: "Windows", darwin: "macOS", linux: "Linux" };

function assetFor(os, arch) {
  return {
    desktop: os === "windows" ? REL_BASE + "jm-desktop_windows_amd64.exe" : null,
    cli: REL_BASE + "jm_" + os + "_" + arch + (os === "windows" ? ".exe" : "")
  };
}

function applyPlatform(os, arch) {
  var deskBtn = document.getElementById("desk-dl");
  var deskNote = document.getElementById("desk-note");
  var assets = assetFor(os, arch);
  var archLabel = arch === "arm64" ? "ARM64" : "x64";

  if (deskBtn) {
    if (assets.desktop) {
      deskBtn.href = assets.desktop;
      deskBtn.textContent = "下载桌面版 · " + OS_LABEL[os] + " " + archLabel;
      if (deskNote) {
        deskNote.hidden = false;
        deskNote.textContent = "已为你的设备匹配安装包，点击即开始下载（v0.6.4）。";
      }
    } else {
      deskBtn.href = assets.cli;
      deskBtn.textContent = "下载 CLI · " + OS_LABEL[os] + " " + archLabel;
      if (deskNote) {
        deskNote.hidden = false;
        deskNote.textContent = "桌面版目前仅提供 Windows 安装包；你的系统（" + OS_LABEL[os] + "）可直接下载对应架构的命令行版。";
      }
    }
  }
  var cliAll = document.querySelectorAll("#cli-all a");
  cliAll.forEach(function (a) {
    a.href = REL_BASE + a.dataset.asset;
    a.classList.toggle("rec", a.dataset.asset === assets.cli.slice(REL_BASE.length));
  });
}

function detectFallback() {
  var ua = navigator.userAgent || "";
  var os = "linux";
  if (/Windows/i.test(ua)) os = "windows";
  else if (/Macintosh|Mac OS X/i.test(ua)) os = "darwin";
  else if (/Android/i.test(ua)) os = "linux";
  var arch = /arm64|aarch64/i.test(ua) ? "arm64" : "amd64";
  applyPlatform(os, arch);
}

if (navigator.userAgentData && navigator.userAgentData.getHighEntropyValues) {
  navigator.userAgentData.getHighEntropyValues(["architecture", "platform"])
    .then(function (h) {
      var p = (h.platform || "").toLowerCase();
      var os = p.indexOf("mac") >= 0 ? "darwin" : p.indexOf("windows") >= 0 ? "windows" : "linux";
      var arch = h.architecture === "arm" ? "arm64" : "amd64";
      applyPlatform(os, arch);
    })
    .catch(detectFallback);
} else {
  detectFallback();
}

var termEl = document.querySelector(".terminal");
if (termEl && !reducedMotion) {
  if ("IntersectionObserver" in window) {
    var tio = new IntersectionObserver(function (entries) {
      entries.forEach(function (en) {
        if (en.isIntersecting) { tio.disconnect(); startTyping(en.target); }
      });
    }, { threshold: 0.35 });
    tio.observe(termEl);
  } else {
    startTyping(termEl);
  }
}
