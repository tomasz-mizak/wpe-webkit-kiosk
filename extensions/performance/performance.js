(function () {
  'use strict';

  var kiosk = window.__kiosk;
  if (!kiosk || !kiosk.overlay) return;

  /* ---- Build panel structure ---- */
  var panel = document.createElement('div');
  panel.id = '__kiosk-perf';
  panel.innerHTML =
    /* Core metrics */
    '<div class="perf-group">' +
    '  <div class="perf-group-title">Core</div>' +
    '  <div class="perf-row"><span class="perf-label">FPS</span><span class="perf-value" id="__p-fps">--</span></div>' +
    '  <div class="perf-bar"><div class="perf-bar-fill" id="__p-fps-bar"></div></div>' +
    '  <div class="perf-row"><span class="perf-label">CPU</span><span class="perf-value" id="__p-cpu">--</span></div>' +
    '  <div class="perf-bar"><div class="perf-bar-fill" id="__p-cpu-bar"></div></div>' +
    '  <div class="perf-row"><span class="perf-label">Load</span><span class="perf-value" id="__p-load">--</span></div>' +
    '</div>' +
    /* Memory */
    '<div class="perf-group">' +
    '  <div class="perf-group-title">Memory</div>' +
    '  <div class="perf-row"><span class="perf-label">RAM</span><span class="perf-value" id="__p-mem">--</span></div>' +
    '  <div class="perf-bar"><div class="perf-bar-fill" id="__p-mem-bar"></div></div>' +
    '  <div class="perf-row"><span class="perf-label">Swap</span><span class="perf-value" id="__p-swap">--</span></div>' +
    '  <div class="perf-row"><span class="perf-label">Process</span><span class="perf-value" id="__p-proc-mem">--</span></div>' +
    '</div>' +
    /* Temperature & GPU */
    '<div class="perf-group">' +
    '  <div class="perf-group-title">Thermal / GPU</div>' +
    '  <div class="perf-row"><span class="perf-label">Temp</span><span class="perf-value" id="__p-temp">--</span></div>' +
    '  <div class="perf-row"><span class="perf-label">GPU</span><span class="perf-value" id="__p-gpu">--</span></div>' +
    '</div>' +
    /* Network */
    '<div class="perf-group">' +
    '  <div class="perf-group-title">Network</div>' +
    '  <div id="__p-net"><span class="perf-muted">loading...</span></div>' +
    '</div>' +
    /* Disk */
    '<div class="perf-group">' +
    '  <div class="perf-group-title">Disk</div>' +
    '  <div id="__p-disk"><span class="perf-muted">loading...</span></div>' +
    '</div>' +
    /* System */
    '<div class="perf-group">' +
    '  <div class="perf-group-title">System</div>' +
    '  <div class="perf-row"><span class="perf-label">Uptime</span><span class="perf-value" id="__p-uptime">--</span></div>' +
    '  <div class="perf-row"><span class="perf-label">PID</span><span class="perf-value" id="__p-pid">--</span></div>' +
    '  <div class="perf-row"><span class="perf-label">Threads</span><span class="perf-value" id="__p-threads">--</span></div>' +
    '</div>' +
    /* WebKit */
    '<div class="perf-group">' +
    '  <div class="perf-group-title">WebKit</div>' +
    '  <div class="perf-row"><span class="perf-label">Page</span><span class="perf-value" id="__p-title" style="max-width:160px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">--</span></div>' +
    '</div>';
  kiosk.overlay.appendChild(panel);

  /* ---- Element refs ---- */
  var el = {};
  var ids = ['fps','fps-bar','cpu','cpu-bar','load','mem','mem-bar','swap',
             'proc-mem','temp','gpu','net','disk','uptime','pid','threads','title'];
  ids.forEach(function (id) { el[id] = document.getElementById('__p-' + id); });

  /* ---- FPS counter ---- */
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

    el.fps.textContent = fps;
    el.fps.className = 'perf-value ' + (fps >= 50 ? 'perf-good' : fps >= 30 ? 'perf-warn' : 'perf-bad');
    var pct = Math.min(fps / 60, 1) * 100;
    el['fps-bar'].style.width = pct + '%';
    el['fps-bar'].style.background = fps >= 50 ? '#4caf50' : fps >= 30 ? '#ff9800' : '#f44336';
  }, 1000);

  /* ---- Helpers ---- */
  function formatBytes(bytes) {
    if (bytes >= 1073741824) return (bytes / 1073741824).toFixed(1) + ' GB';
    if (bytes >= 1048576) return (bytes / 1048576).toFixed(0) + ' MB';
    if (bytes >= 1024) return (bytes / 1024).toFixed(0) + ' KB';
    return bytes + ' B';
  }

  function formatUptime(sec) {
    var d = Math.floor(sec / 86400);
    var h = Math.floor((sec % 86400) / 3600);
    var m = Math.floor((sec % 3600) / 60);
    if (d > 0) return d + 'd ' + h + 'h';
    if (h > 0) return h + 'h ' + m + 'm';
    return m + 'm';
  }

  function colorByPct(pct) {
    return pct <= 50 ? 'perf-good' : pct <= 80 ? 'perf-warn' : 'perf-bad';
  }

  /* ---- Previous network counters for rate calculation ---- */
  var prevNet = {};
  var prevNetTime = 0;

  /* ---- CPU state ---- */
  var prevIdle = 0;
  var prevTotal = 0;

  /* ---- Stats polling ---- */
  function updateStats() {
    kiosk.sendMessage('getStats').then(function (response) {
      var s = JSON.parse(response);

      /* CPU */
      if (s.cpuLine) {
        var parts = s.cpuLine.split(/\s+/).slice(1).map(Number);
        var idle = parts[3] + (parts[4] || 0);
        var total = parts.reduce(function (a, b) { return a + b; }, 0);
        var di = idle - prevIdle;
        var dt = total - prevTotal;
        prevIdle = idle;
        prevTotal = total;
        if (dt > 0) {
          var usage = Math.round(((dt - di) / dt) * 100);
          el.cpu.textContent = usage + '%';
          el.cpu.className = 'perf-value ' + colorByPct(usage);
          el['cpu-bar'].style.width = usage + '%';
          el['cpu-bar'].style.background = usage <= 50 ? '#4caf50' : usage <= 80 ? '#ff9800' : '#f44336';
        }
      }

      /* Load average */
      if (s.loadAvg) {
        el.load.textContent = s.loadAvg.map(function (v) { return v.toFixed(2); }).join(' / ');
      }

      /* Memory */
      if (s.memTotalKB && s.memAvailableKB) {
        var usedMB = Math.round((s.memTotalKB - s.memAvailableKB) / 1024);
        var totalMB = Math.round(s.memTotalKB / 1024);
        var memPct = Math.round(((s.memTotalKB - s.memAvailableKB) / s.memTotalKB) * 100);
        el.mem.textContent = usedMB + ' / ' + totalMB + ' MB';
        el.mem.className = 'perf-value ' + colorByPct(memPct);
        el['mem-bar'].style.width = memPct + '%';
        el['mem-bar'].style.background = memPct <= 50 ? '#4caf50' : memPct <= 80 ? '#ff9800' : '#f44336';
      }

      /* Swap */
      if (s.swapTotalKB !== undefined) {
        if (s.swapTotalKB === 0) {
          el.swap.textContent = 'none';
          el.swap.className = 'perf-value perf-muted';
        } else {
          var swapUsed = Math.round((s.swapTotalKB - s.swapFreeKB) / 1024);
          var swapTotal = Math.round(s.swapTotalKB / 1024);
          el.swap.textContent = swapUsed + ' / ' + swapTotal + ' MB';
        }
      }

      /* Process memory */
      if (s.process) {
        el['proc-mem'].textContent = Math.round(s.process.vmRssKB / 1024) + ' MB RSS';
        el.pid.textContent = s.process.pid;
        el.threads.textContent = s.process.threads;
      }

      /* Temperature */
      if (s.temperatures && s.temperatures.length > 0) {
        var maxTemp = 0;
        s.temperatures.forEach(function (t) { if (t.tempC > maxTemp) maxTemp = t.tempC; });
        el.temp.textContent = maxTemp.toFixed(0) + '\u00B0C';
        el.temp.className = 'perf-value ' + (maxTemp <= 60 ? 'perf-good' : maxTemp <= 80 ? 'perf-warn' : 'perf-bad');
      }

      /* GPU */
      if (s.gpu && s.gpu.freqMHz !== undefined) {
        var gpuText = s.gpu.freqMHz + ' MHz';
        if (s.gpu.maxFreqMHz) gpuText += ' / ' + s.gpu.maxFreqMHz;
        el.gpu.textContent = gpuText;
      }

      /* Network (with rate calculation) */
      if (s.network) {
        var now = Date.now();
        var html = '';
        s.network.forEach(function (n) {
          var rate = '';
          if (prevNet[n.iface] && prevNetTime > 0) {
            var dtSec = (now - prevNetTime) / 1000;
            var rxRate = (n.rxBytes - prevNet[n.iface].rx) / dtSec;
            var txRate = (n.txBytes - prevNet[n.iface].tx) / dtSec;
            rate = ' <span class="perf-muted">\u2193' + formatBytes(Math.round(rxRate)) +
                   '/s \u2191' + formatBytes(Math.round(txRate)) + '/s</span>';
          }
          prevNet[n.iface] = { rx: n.rxBytes, tx: n.txBytes };
          html += '<div class="perf-row"><span class="perf-label">' + n.iface +
                  '</span><span class="perf-value">' +
                  formatBytes(n.rxBytes) + rate + '</span></div>';
        });
        prevNetTime = now;
        el.net.innerHTML = html || '<span class="perf-muted">no interfaces</span>';
      }

      /* Disk */
      if (s.disk) {
        var dhtml = '';
        s.disk.forEach(function (d) {
          var usedPct = Math.round(((d.totalBytes - d.availBytes) / d.totalBytes) * 100);
          dhtml += '<div class="perf-row"><span class="perf-label">' + d.mount +
                   '</span><span class="perf-value ' + colorByPct(usedPct) + '">' +
                   formatBytes(d.totalBytes - d.availBytes) + ' / ' + formatBytes(d.totalBytes) +
                   ' (' + usedPct + '%)</span></div>';
        });
        el.disk.innerHTML = dhtml;
      }

      /* Uptime */
      if (s.uptimeSec !== undefined) {
        el.uptime.textContent = formatUptime(s.uptimeSec);
      }

      /* WebKit page */
      if (s.webkit) {
        el.title.textContent = s.webkit.title || s.webkit.uri || '--';
        el.title.title = s.webkit.uri || '';
      }

    }).catch(function () {
      el.cpu.textContent = 'N/A';
      el.mem.textContent = 'N/A';
    });
  }

  setInterval(updateStats, 2000);
  updateStats();
})();
