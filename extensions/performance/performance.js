(function () {
  'use strict';

  var kiosk = window.__kiosk;
  if (!kiosk || !kiosk.overlay) return;

  /* ---- Create panel ---- */
  var panel = document.createElement('div');
  panel.id = '__kiosk-perf';
  panel.innerHTML =
    '<div class="perf-section">' +
    '  <div class="perf-label">FPS</div>' +
    '  <div class="perf-value" id="__perf-fps">--</div>' +
    '  <div class="perf-bar"><div class="perf-bar-fill" id="__perf-fps-bar"></div></div>' +
    '</div>' +
    '<div class="perf-section">' +
    '  <div class="perf-label">CPU</div>' +
    '  <div class="perf-value" id="__perf-cpu">--</div>' +
    '  <div class="perf-bar"><div class="perf-bar-fill" id="__perf-cpu-bar"></div></div>' +
    '</div>' +
    '<div class="perf-section">' +
    '  <div class="perf-label">Memory</div>' +
    '  <div class="perf-value" id="__perf-mem">--</div>' +
    '</div>';
  kiosk.overlay.appendChild(panel);

  var fpsEl = document.getElementById('__perf-fps');
  var fpsBar = document.getElementById('__perf-fps-bar');
  var cpuEl = document.getElementById('__perf-cpu');
  var cpuBar = document.getElementById('__perf-cpu-bar');
  var memEl = document.getElementById('__perf-mem');

  /* ---- FPS counter (client-side, no native call needed) ---- */
  var frames = 0;
  var lastTime = performance.now();

  function countFrame() {
    frames++;
    requestAnimationFrame(countFrame);
  }
  requestAnimationFrame(countFrame);

  setInterval(function () {
    var now = performance.now();
    var elapsed = now - lastTime;
    var fps = Math.round((frames * 1000) / elapsed);
    frames = 0;
    lastTime = now;

    fpsEl.textContent = fps + ' fps';
    var pct = Math.min(fps / 60, 1) * 100;
    fpsBar.style.width = pct + '%';
    fpsBar.style.background = fps >= 50 ? '#4caf50' : fps >= 30 ? '#ff9800' : '#f44336';
  }, 1000);

  /* ---- CPU + Memory (native /proc reading via message handler) ---- */
  var prevIdle = 0;
  var prevTotal = 0;

  function updateStats() {
    kiosk.sendMessage('getStats').then(function (response) {
      var stats = JSON.parse(response);

      /* CPU from /proc/stat first line */
      if (stats.cpuLine) {
        var parts = stats.cpuLine.split(/\s+/).slice(1).map(Number);
        var idle = parts[3] + (parts[4] || 0);
        var total = parts.reduce(function (a, b) { return a + b; }, 0);
        var diffIdle = idle - prevIdle;
        var diffTotal = total - prevTotal;
        prevIdle = idle;
        prevTotal = total;

        if (diffTotal > 0) {
          var usage = Math.round(((diffTotal - diffIdle) / diffTotal) * 100);
          cpuEl.textContent = usage + '%';
          cpuBar.style.width = usage + '%';
          cpuBar.style.background = usage <= 50 ? '#4caf50' : usage <= 80 ? '#ff9800' : '#f44336';
        }
      }

      /* Memory from /proc/meminfo */
      if (stats.memTotalKB && stats.memAvailableKB) {
        var usedMB = Math.round((stats.memTotalKB - stats.memAvailableKB) / 1024);
        var totalMB = Math.round(stats.memTotalKB / 1024);
        memEl.textContent = usedMB + ' / ' + totalMB + ' MB';
      }
    }).catch(function () {
      cpuEl.textContent = 'N/A';
      memEl.textContent = 'N/A';
    });
  }

  setInterval(updateStats, 2000);
  updateStats();
})();
