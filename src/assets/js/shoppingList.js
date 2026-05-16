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
    var currentQty = parseInt(qtyEl.textContent, 10) || 0;
    var newQty = Math.max(1, currentQty + delta);
    qtyEl.textContent = newQty;
    fetch('/api/v1/shopping-list/' + itemId, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ quantity: newQty })
    }).then(function(res) {
      if (!res.ok) {
        qtyEl.textContent = currentQty;
        proviant.showFeedback('error', 'Update failed');
      }
    }).catch(function() {
      qtyEl.textContent = currentQty;
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
        proviant.showFeedback('error', 'Failed to save notes');
      }
    }).catch(function() {
      proviant.showFeedback('error', 'Failed to save notes');
    });
  }

  function deleteItem(itemId) {
    if (!confirm('Delete this item?')) return;
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
        proviant.showFeedback('error', 'Delete failed');
      }
    }).catch(function() {
      proviant.showFeedback('error', 'Delete failed');
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
      proviant.showFeedback('error', 'Toggle failed');
    });
  }

  function addItem() {
    var nameInput = document.getElementById('inputItemName');
    var qtyInput = document.getElementById('inputItemQuantity');
    var unitInput = document.getElementById('inputItemUnit');
    if (!nameInput || !nameInput.value.trim()) {
      nameInput.focus();
      return;
    }
    var payload = {
      name: nameInput.value.trim(),
      quantity: parseInt(qtyInput ? qtyInput.value : '1', 10) || 1,
      unit: unitInput ? unitInput.value.trim() : ''
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
      if (unitInput) unitInput.value = '';
      nameInput.focus();
      proviant.showFeedback('success', 'Item added');
      setTimeout(function() { location.reload(); }, 300);
    }).catch(function() {
      proviant.showFeedback('error', 'Failed to add item');
    });
  }

  function importAutoItems() {
    var btn = document.getElementById('btnImportAll');
    if (btn) {
      btn.disabled = true;
      btn.innerHTML = '<span class="spinner-border spinner-border-sm me-1"></span>Importing...';
    }
    fetch('/api/v1/shopping-list/import-auto', {
      method: 'POST'
    }).then(function(res) {
      return res.json();
    }).then(function(data) {
      proviant.showFeedback('success', data.message || 'Items imported');
      setTimeout(function() { location.reload(); }, 500);
    }).catch(function() {
      if (btn) {
        btn.disabled = false;
        btn.innerHTML = 'Import all';
      }
      proviant.showFeedback('error', 'Import failed');
    });
  }

  function updateItemCount() {
    var container = document.getElementById('shoppingListContainer');
    if (!container) return;
    var rows = container.querySelectorAll('.shopping-list-item');
    var total = rows.length;
    var subtitle = document.querySelector('.page-header-subtitle');
    if (subtitle) {
      subtitle.textContent = total + ' item' + (total !== 1 ? 's' : '') + ' in your list';
    }
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

    var html = '<!DOCTYPE html><html><head><meta charset="utf-8"><title>Shopping List</title><style>';
    html += 'body{font-family:system-ui,sans-serif;padding:1rem;color:#221e18}';
    html += 'h1{margin:0 0 0.5rem;font-size:1.5rem}';
    html += '.meta{margin-bottom:1rem;font-size:0.875rem;color:#666}';
    html += '.item{display:flex;flex-direction:column;padding:0.5rem 0;border-bottom:1px solid #dad6d0}';
    html += '.item-row{display:flex;align-items:center}';
    html += '.checkbox{width:18px;height:18px;border:2px solid #221e18;border-radius:3px;margin-right:0.5rem;flex-shrink:0}';
    html += '.name{font-size:1.1rem;flex:1}';
    html += '.amount{font-size:1rem;color:#4a6fa0;margin-left:0.5rem}';
    html += '.notes{font-size:0.875rem;color:#888;margin-left:2.25rem;margin-top:0.25rem}';
    html += '</style></head><body>';
    html += '<h1>Shopping List</h1>';
    html += '<p class="meta">' + items.length + ' items</p>';
    items.forEach(function(item) {
      var name = item.name || item.Name || '';
      var qty = item.quantity || item.Quantity || '';
      var unit = item.unit || item.Unit || '';
      var checked = item.checked || item.Checked ? ' checked' : '';
      var notes = item.notes || item.Notes || '';
      html += '<div class="item' + checked + '">';
      html += '<div class="item-row">';
      html += '<div class="checkbox"></div>';
      html += '<span class="name">' + name + '</span>';
      if (qty) {
        html += '<span class="amount">' + qty + (unit ? ' ' + unit : '') + '</span>';
      }
      html += '</div>';
      if (notes) {
        html += '<div class="notes">' + notes + '</div>';
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
