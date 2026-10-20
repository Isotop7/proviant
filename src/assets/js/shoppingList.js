'use strict';

(function() {
  var debounceTimer = null;

  document.addEventListener('DOMContentLoaded', function () {
    document.addEventListener('click', function (event) {
      var target = event.target.closest('[data-qty-action][data-item-id]');
      if (target) {
        var itemId = target.dataset.itemId;
        var action = target.dataset.qtyAction;
        var delta = action === 'inc' ? 1 : -1;
        updateItemQuantity(itemId, delta);
        return;
      }

      var stepBtn = event.target.closest('[data-item-quantity-step]');
      if (stepBtn) {
        stepQuantityInput(Number.parseInt(stepBtn.dataset.itemQuantityStep, 10) || 0);
        return;
      }

      var delBtn = event.target.closest('[data-delete-item]');
      if (delBtn) {
        var delId = delBtn.dataset.deleteItem;
        deleteItem(delId);
        return;
      }

      var notesToggle = event.target.closest('[data-notes-toggle]');
      if (notesToggle) {
        var notesId = notesToggle.dataset.notesToggle;
        var notesEl = document.getElementById('notes-' + notesId);
        if (notesEl) {
          notesEl.classList.toggle('expanded');
        }
        return;
      }

      var importBtn = event.target.closest('#btnImportAll');
      if (importBtn) {
        importAutoItems();
        return;
      }

      var addBtn = event.target.closest('#btnAddItem');
      if (addBtn) {
        addItem();
        return;
      }

      var checkbox = event.target.closest('.item-checkbox');
      if (checkbox && checkbox.dataset.itemId) {
        toggleItem(checkbox.dataset.itemId);
        return;
      }

      var printBtn = event.target.closest('#btnPrintShoppingList');
      if (printBtn) {
        printShoppingList();
        return;
      }
    });

    document.addEventListener('input', function (event) {
      var notesInput = event.target.closest('[data-notes-input]');
      if (notesInput) {
        var itemId = notesInput.dataset.notesInput;
        clearTimeout(debounceTimer);
        debounceTimer = setTimeout(function() {
          updateItemNotes(itemId, notesInput.value);
        }, 500);
      }
    });

    var nameInput = document.getElementById('inputItemName');
    if (nameInput) {
      nameInput.addEventListener('keydown', function (event) {
        if (event.key === 'Enter') {
          event.preventDefault();
          addItem();
        }
      });
    }
  });

  function updateItemQuantity(itemId, delta) {
    var qtyEl = document.getElementById('qty-' + itemId);
    if (!qtyEl) return;
    // The count lives in the data attribute. The text is the rendered label
    // ("2 Stk.") and must not be parsed: a locale that does not separate the
    // number from the noun would yield NaN.
    var currentQty = Number.parseInt(qtyEl.dataset.quantity, 10) || 0;
    var newQty = Math.max(1, currentQty + delta);
    qtyEl.dataset.quantity = String(newQty);
    proviant.renderQuantityLabel(qtyEl);
    fetch('/api/v1/shopping-list/' + itemId, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ quantity: newQty })
    }).then(function(res) {
      if (!res.ok) {
        qtyEl.dataset.quantity = String(currentQty);
        proviant.renderQuantityLabel(qtyEl);
        proviant.showFeedback('error', proviant.t("js.shoppingList.updateFailedTitle"));
      }
    }).catch(function() {
      qtyEl.dataset.quantity = String(currentQty);
      proviant.renderQuantityLabel(qtyEl);
    });
  }

  function updateItemNotes(itemId, notes) {
    var notesInput = document.querySelector('[data-notes-input="' + itemId + '"]');
    fetch('/api/v1/shopping-list/' + itemId, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ notes: notes })
    }).then(function(res) {
      if (res.ok && notesInput) {
        notesInput.classList.add('notes-saved');
        setTimeout(function() {
          notesInput.classList.remove('notes-saved');
        }, 1200);
      } else if (!res.ok) {
        proviant.showFeedback('error', proviant.t("js.shoppingList.saveNotesFailedTitle"));
      }
    }).catch(function() {
      proviant.showFeedback('error', proviant.t("js.shoppingList.saveNotesFailedTitle"));
    });
  }

  function deleteItem(itemId) {
    proviant.showConfirm(proviant.t("js.shoppingList.deleteConfirmTitle"), proviant.t("js.shoppingList.deleteConfirmBody"), function() {
      doDeleteItem(itemId);
    }, proviant.t("common.delete"), 'danger');
  }

  function doDeleteItem(itemId) {
    fetch('/api/v1/shopping-list/' + itemId, {
      method: 'DELETE'
    }).then(function(res) {
      if (res.ok) {
        var el = document.getElementById('item-' + itemId);
        if (el) {
          el.style.transition = 'opacity 0.3s ease-out';
          el.style.opacity = '0';
          setTimeout(function() { el.remove(); }, 300);
        }
        var notesEl = document.getElementById('notes-' + itemId);
        if (notesEl) notesEl.remove();
        updateItemCount();
      } else {
        proviant.showFeedback('error', proviant.t("js.shoppingList.deleteFailedTitle"));
      }
    }).catch(function() {
      proviant.showFeedback('error', proviant.t("js.shoppingList.deleteFailedTitle"));
    });
  }

  function toggleItem(itemId) {
    fetch('/api/v1/shopping-list/' + itemId + '/toggle', {
      method: 'POST'
    }).then(function(res) {
      return res.json();
    }).then(function(data) {
      var el = document.getElementById('item-' + itemId);
      if (!el) return;
      if (data.checked) {
        el.classList.add('checked');
      } else {
        el.classList.remove('checked');
      }
      updateItemCount();
    }).catch(function() {
      proviant.showFeedback('error', proviant.t("js.shoppingList.toggleFailedTitle"));
    });
  }

  // The add form's stepper moves the count input by one pack, never below one.
  function stepQuantityInput(delta) {
    var input = document.getElementById('inputItemQuantity');
    if (!input) return;
    var current = Number.parseInt(input.value, 10) || 1;
    input.value = String(Math.max(1, current + delta));
  }

  function addItem() {
    var nameInput = document.getElementById('inputItemName');
    var qtyInput = document.getElementById('inputItemQuantity');
    if (!nameInput || !nameInput.value.trim()) {
      nameInput.focus();
      return;
    }
    var payload = {
      name: nameInput.value.trim(),
      quantity: Number.parseInt(qtyInput ? qtyInput.value : '1', 10) || 1
    };
    fetch('/api/v1/shopping-list', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    }).then(function(res) {
      return res.json();
    }).then(function() {
      nameInput.value = '';
      if (qtyInput) qtyInput.value = '1';
      nameInput.focus();
      proviant.showFeedback('success', proviant.t("js.shoppingList.itemAddedTitle"));
      setTimeout(function() { location.reload(); }, 300);
    }).catch(function() {
      proviant.showFeedback('error', proviant.t("js.shoppingList.addItemFailedTitle"));
    });
  }

  function importAutoItems() {
    var btn = document.getElementById('btnImportAll');
    if (btn) {
      btn.disabled = true;
      btn.innerHTML = '<span class="spinner-border spinner-border-sm me-1"></span>' + proviant.t("js.shoppingList.importing");
    }
    fetch('/api/v1/shopping-list/import-auto', {
      method: 'POST'
    }).then(function(res) {
      return res.json();
    }).then(function(data) {
      proviant.showFeedback('success', data.message || proviant.t("js.shoppingList.itemsImportedTitle"));
      setTimeout(function() { location.reload(); }, 500);
    }).catch(function() {
      if (btn) {
        btn.disabled = false;
        btn.innerHTML = proviant.t("js.shoppingList.importAll");
      }
      proviant.showFeedback('error', proviant.t("js.shoppingList.importFailedTitle"));
    });
  }

  function updateItemCount() {
    var container = document.getElementById('shoppingListContainer');
    if (!container) return;
    var rows = container.querySelectorAll('.shopping-list-item');
    var total = rows.length;
    var subtitle = document.querySelector('.page-header-subtitle');
    if (subtitle) {
      subtitle.textContent = proviant.tn("js.shoppingList.itemCount", total);
    }
  }

  // The print view is written into a same-origin window, so every value that
  // reaches the markup has to be escaped: an item name is user input.
  function escapePrintHtml(text) {
    if (!text) return '';
    var div = document.createElement('div');
    div.textContent = String(text);
    return div.innerHTML.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
  }

  function printShoppingList() {
    var dataEl = document.getElementById('shoppingListData');
    if (!dataEl) return;
    var rawText = (dataEl.textContent || '').trim();
    if (!rawText) return;
    var items = [];
    try {
      var parsed = JSON.parse(rawText);
      // If parsed is a string, try parsing it as JSON (handles template quoting issue)
      if (typeof parsed === 'string') {
        parsed = JSON.parse(parsed);
      }
      if (Array.isArray(parsed)) {
        items = parsed;
      } else if (parsed && typeof parsed === 'object') {
        items = [parsed];
      }
    } catch {
      return;
    }
    if (!items.length) return;

    var html = '<!DOCTYPE html><html><head><meta charset="utf-8"><title>' + proviant.t("nav.shoppingList") + '</title><style>';
    html += 'body{font-family:system-ui,sans-serif;padding:1rem;color:#221e18}';
    html += 'h1{margin:0 0 0.5rem;font-size:1.5rem}';
    html += '.meta{margin-bottom:1rem;font-size:0.875rem;color:#666}';
    html += '.item{display:flex;flex-direction:column;padding:0.5rem 0;border-bottom:1px solid #dad6d0}';
    html += '.item-row{display:flex;align-items:center}';
    html += '.checkbox{width:18px;height:18px;border:2px solid #221e18;border-radius:3px;margin-right:0.5rem;flex-shrink:0}';
    html += '.name{font-size:1.1rem;flex:1}';
    html += '.qty{font-size:1rem;color:#4a6fa0;margin-left:0.5rem}';
    html += '.pack-size{font-size:0.875rem;color:#666;margin-left:0.35rem}';
    html += '.notes{font-size:0.875rem;color:#888;margin-left:2.25rem;margin-top:0.25rem}';
    html += '</style></head><body>';
    html += '<h1>' + proviant.t("nav.shoppingList") + '</h1>';
    html += '<p class="meta">' + proviant.tn("js.shoppingList.printCount", items.length) + '</p>';
    items.forEach(function(item) {
      var name = item.name || item.Name || '';
      // Print keeps the stored count, not the row's "2 Stk." label: at the store you need how many to buy.
      var qty = item.quantity || item.Quantity || '';
      var packSize = item.packSize || item.PackSize || '';
      var checked = item.checked || item.Checked ? ' checked' : '';
      var notes = item.notes || item.Notes || '';
      html += '<div class="item' + checked + '">';
      html += '<div class="item-row">';
      html += '<div class="checkbox"></div>';
      html += '<span class="name">' + escapePrintHtml(name) + '</span>';
      if (qty) {
        html += '<span class="qty">' + escapePrintHtml(qty) + '</span>';
      }
      if (packSize) {
        html += '<span class="pack-size">' + escapePrintHtml(packSize) + '</span>';
      }
      html += '</div>';
      if (notes) {
        html += '<div class="notes">' + escapePrintHtml(notes) + '</div>';
      }
      html += '</div>';
    });
    html += '</body></html>';

    var printWindow = window.open('', '', 'width=600,height=400');
    if (!printWindow) return;
    printWindow.document.write(html);
    printWindow.document.close();
    printWindow.focus();
    printWindow.print();
  }
})();
