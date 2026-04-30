// Minimal JS for ReadItLater UI: refetch handling and error dialog
(function(){
  function $(sel, ctx){ return (ctx||document).querySelector(sel); }
  function $all(sel, ctx){ return Array.from((ctx||document).querySelectorAll(sel)); }

  function showDialog(dialog){
    if (!dialog) return;
    try{ if (dialog.showModal) dialog.showModal(); else dialog.setAttribute('open', ''); }catch(e){ dialog.setAttribute('open', ''); }
  }
  function closeDialog(dialog){ if (!dialog) return; try{ if (dialog.close) dialog.close(); else dialog.removeAttribute('open'); }catch(e){ dialog.removeAttribute('open'); } }

  document.addEventListener('DOMContentLoaded', function(){
    // refetch button
    var refetchBtn = $('#refetch-btn');
    var refetchDialog = $('#refetch-dialog');
    var refetchMsg = refetchDialog && refetchDialog.querySelector('#refetch-message');
    var refetchClose = refetchDialog && refetchDialog.querySelector('#refetch-close');

    if (refetchClose) refetchClose.addEventListener('click', function(){ closeDialog(refetchDialog); });

    if (refetchBtn){
      refetchBtn.addEventListener('click', function(){
        var urlRaw = refetchBtn.getAttribute('data-url') || '';
        if (!urlRaw) return;
        refetchBtn.disabled = true;
        // show a small inline spinner
        var spinner = document.createElement('span'); spinner.className = 'spinner'; spinner.id = 'refetch-spinner';
        refetchBtn.appendChild(spinner);

        var encoded = encodeURIComponent(urlRaw);
        var fetchUrl = '/?url=' + encoded + '&force=1';
        fetch(fetchUrl, { method: 'GET', credentials: 'same-origin' }).then(function(resp){
          if (resp.ok){
            // navigate to fresh result without force
            window.location.href = '/?url=' + encoded;
            return;
          }
          return resp.text().then(function(txt){ throw new Error(txt || ('HTTP ' + resp.status)); });
        }).catch(function(err){
          console.error('Refetch failed', err);
          var msg = String(err && err.message ? err.message : err);
          if (msg.indexOf('Fehler:') === 0) msg = msg.replace(/^Fehler:\s*/i, '');
          if (refetchMsg) refetchMsg.textContent = msg || 'Fehler beim Neuladen';
          showDialog(refetchDialog);
        }).finally(function(){
          refetchBtn.disabled = false;
          var sp = document.getElementById('refetch-spinner'); if (sp) sp.remove();
        });
      });
    }

    // error dialog handling from server-rendered dataset
    var body = document.body;
    var errorMessage = body && body.getAttribute('data-error-message');
    var errorDialog = $('#error-dialog');
    var errorMsgEl = $('#error-message');
    var errorOpen = $('#error-open');
    var errorClose = $('#error-close');
    if (errorMessage && errorDialog){
      if (errorMsgEl) errorMsgEl.textContent = errorMessage;
      if (errorOpen) errorOpen.addEventListener('click', function(){ var url = body.getAttribute('data-error-url') || window.location.href; window.open(url, '_blank'); });
      if (errorClose) errorClose.addEventListener('click', function(){ closeDialog(errorDialog); });
      showDialog(errorDialog);
    }
  });
})();
