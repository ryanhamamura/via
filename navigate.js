(function() {
  const meta = document.querySelector('meta[data-signals]');
  if (!meta) return;
  const raw = meta.getAttribute('data-signals');
  const parsed = JSON.parse(raw.replace(/'/g, '"'));
  const ctxID = parsed['via-ctx'];
  const csrf = parsed['via-csrf'];
  if (!ctxID || !csrf) return;

  function navigate(url, popstate) {
    const params = new URLSearchParams({
      'via-ctx': ctxID,
      'via-csrf': csrf,
      'url': url,
    });
    if (popstate) params.set('popstate', '1');
    fetch('/_navigate', {
      method: 'POST',
      headers: {'Content-Type': 'application/x-www-form-urlencoded'},
      body: params.toString()
    }).then(function(res) {
      if (!res.ok) window.location.href = url;
    }).catch(function() {
      window.location.href = url;
    });
  }

  document.addEventListener('click', function(e) {
    var el = e.target;
    while (el && el.tagName !== 'A') el = el.parentElement;
    if (!el) return;
    if (e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return;
    if (el.hasAttribute('target')) return;
    if (el.hasAttribute('data-via-no-boost')) return;
    var href = el.getAttribute('href');
    if (!href || href.startsWith('#')) return;
    try {
      var url = new URL(href, window.location.origin);
      if (url.origin !== window.location.origin) return;
      e.preventDefault();
      navigate(url.pathname + url.search + url.hash);
    } catch(_) {}
  });

  var ready = false;
  window.addEventListener('popstate', function() {
    if (!ready) return;
    navigate(window.location.pathname + window.location.search + window.location.hash, true);
  });
  setTimeout(function() { ready = true; }, 0);
})();
