'use strict';

document.addEventListener('DOMContentLoaded', function () {
  document.addEventListener('click', function (event) {
    var btn = event.target.closest('[data-qty-action][data-product-id]');
    if (!btn) return;

    var productID = btn.dataset.productId;
    var action = btn.dataset.qtyAction;
    var delta = action === 'inc' ? 1 : -1;

    proviant.updateProductAmount(productID, delta).then(function (result) {
      if (result.code !== 200) {
        proviant.showFeedback('error', 'Update failed', result.message);
        return;
      }

      var qtyEl = document.getElementById('qty-' + productID);
      if (qtyEl) {
        var newQty = parseInt(qtyEl.textContent, 10) + delta;
        qtyEl.textContent = newQty;
      }

      if (result.deleted || (result.message && result.message.includes('threshold'))) {
        var row = document.getElementById('card-' + productID);
        if (row) {
          row.style.transition = 'opacity 0.4s ease-out';
          row.style.opacity = '0';
          setTimeout(function () { row.remove(); }, 400);
        }
      }
    });
  });

  var printBtn = document.getElementById('btnPrintShoppingList');
  if (printBtn) {
    printBtn.addEventListener('click', function () {
      var dataEl = document.getElementById('shoppingListData');
      if (!dataEl) return;
      var items;
      try {
        items = JSON.parse(dataEl.textContent);
      } catch (e) {
        return;
      }
      if (!items || !items.length) return;

      var html = '<!DOCTYPE html><html><head><meta charset="utf-8"><title>Shopping List</title><style>';
      html += 'body{font-family:system-ui,sans-serif;padding:1rem;color:#221e18}';
      html += 'h1{margin:0 0 0.5rem;font-size:1.5rem}';
      html += '.meta{margin-bottom:1rem;font-size:0.875rem;color:#666}';
      html += '.item{display:flex;align-items:center;padding:0.5rem 0;border-bottom:1px solid #dad6d0}';
      html += '.checkbox{width:18px;height:18px;border:2px solid #221e18;border-radius:3px;margin-right:0.5rem;flex-shrink:0}';
      html += '.name{font-size:1.1rem;flex:1}';
      html += '.amount{font-size:1rem;color:#4a6fa0;margin-left:0.5rem}';
      html += '</style></head><body>';
      html += '<h1>Shopping List</h1>';
      html += '<p class="meta">' + items.length + ' items below stock threshold</p>';
      items.forEach(function (item) {
        html += '<div class="item">';
        html += '<div class="checkbox"></div>';
        html += '<span class="name">' + item.name + '</span>';
        html += '<span class="amount">' + item.amount + ' / ' + item.minAmount + '</span>';
        html += '</div>';
      });
      html += '</body></html>';

      var printWindow = window.open('', '', 'width=600,height=400');
      printWindow.document.write(html);
      printWindow.document.close();
      printWindow.focus();
      printWindow.print();
    });
  }
});