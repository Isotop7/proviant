'use strict';

(function() {
  var debounceTimer = null;
  var purchaseItemId = null;
  var categoryTouched = false;

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

      var buyBtn = event.target.closest('[data-buy-item]');
      if (buyBtn) {
        openPurchaseDialog(buyBtn.dataset.buyItem);
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
        return;
      }
      // A date the user set is theirs: a later suggestion must not overwrite
      // it.
      if (event.target.id === 'purchaseExpiry') {
        event.target.dataset.userEdited = '1';
        delete event.target.dataset.suggested;
      }
    });

    document.addEventListener('change', function (event) {
      if (event.target.id === 'purchaseExpiry') {
        event.target.dataset.userEdited = '1';
        delete event.target.dataset.suggested;
        return;
      }
      if (event.target.id === 'purchaseCategory') {
        categoryTouched = true;
        applyPurchaseExpiry();
      }
    });

    var purchaseForm = document.getElementById('purchaseForm');
    if (purchaseForm) {
      purchaseForm.addEventListener('submit', function (event) {
        event.preventDefault();
        submitPurchase();
      });
    }

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
      btn.innerHTML = '<span class="spinner-border spinner-sm me-1"></span>' + proviant.t("js.shoppingList.importing");
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
    var rows = container.querySelectorAll('.shopping-list__item');
    var count = document.querySelector('.page-header__count');
    if (count) {
      count.textContent = String(rows.length);
    }
    var checked = container.querySelectorAll('.shopping-list__item.checked');
    var meta = document.querySelector('.page-header__meta');
    if (meta) {
      meta.textContent = proviant.t("shoppingList.checkedOff", checked.length);
    }
  }

  // The items the page was rendered with. The rows are read from here and
  // not from the DOM, so a row that was checked or deleted meanwhile cannot
  // be bought by accident.
  function shoppingListItems() {
    var dataEl = document.getElementById('shoppingListData');
    if (!dataEl) return [];
    var rawText = (dataEl.textContent || '').trim();
    if (!rawText) return [];
    try {
      var parsed = JSON.parse(rawText);
      // If parsed is a string, try parsing it as JSON (handles template quoting issue)
      if (typeof parsed === 'string') {
        parsed = JSON.parse(parsed);
      }
      if (Array.isArray(parsed)) {
        return parsed;
      }
      if (parsed && typeof parsed === 'object') {
        return [parsed];
      }
    } catch {
      return [];
    }
    return [];
  }

  // The household's categories with the shelf life the household gave them.
  function householdCategories() {
    var dataEl = document.getElementById('shoppingListCategories');
    if (!dataEl) return [];
    var rawText = (dataEl.textContent || '').trim();
    if (!rawText) return [];
    try {
      var parsed = JSON.parse(rawText);
      return Array.isArray(parsed) ? parsed : [];
    } catch {
      return [];
    }
  }

  // The id of an entry of the embedded payload. The model does not tag the
  // primary key, so it travels under its Go name.
  function itemID(entry) {
    if (!entry) return null;
    return entry.id !== undefined && entry.id !== null ? entry.id : entry.ID;
  }

  // The product an entry of the embedded payload points at. The model does
  // not tag the column either.
  function itemProductID(entry) {
    if (!entry) return null;
    var value = entry.ProductID !== undefined && entry.ProductID !== null ? entry.ProductID : entry.productId;
    return value === undefined ? null : value;
  }

  function openPurchaseDialog(itemId) {
    var item = shoppingListItems().find(function (entry) {
      return String(itemID(entry)) === String(itemId);
    });
    if (!item) {
      proviant.showFeedback('error', proviant.t("js.shoppingList.purchaseFailedTitle"));
      return;
    }
    purchaseItemId = itemID(item);

    var nameEl = document.getElementById('purchaseItemName');
    if (nameEl) nameEl.textContent = item.name || '';

    var quantityEl = document.getElementById('purchaseQuantity');
    if (quantityEl) quantityEl.value = String(item.quantity || 1);

    var packSizeEl = document.getElementById('purchasePackSize');
    if (packSizeEl) packSizeEl.value = item.packSize || '';

    var barcodeEl = document.getElementById('purchaseBarcode');
    if (barcodeEl) barcodeEl.value = '';

    var expiryEl = document.getElementById('purchaseExpiry');
    if (expiryEl) {
      expiryEl.value = '';
      delete expiryEl.dataset.userEdited;
      delete expiryEl.dataset.suggested;
    }

    var categories = householdCategories();
    var categoryEl = document.getElementById('purchaseCategory');
    fillCategoryOptions(categoryEl, categories);
    var categoryKey = item.categoryKey || proviant.resolveCategoryKey(item.category, categories);
    if (categoryEl) categoryEl.value = categoryKey || '';
    categoryTouched = false;

    // A product the household already tracks keeps its own date, category,
    // pack size and location: the purchase only adds packs to it. The fields
    // that would be ignored are hidden rather than filled with a value the
    // server drops.
    var linked = itemProductID(item);
    var existingEl = document.getElementById('purchaseExistingProduct');
    if (existingEl) {
      existingEl.textContent = linked
        ? proviant.t("js.shoppingList.purchaseIntoExisting", item.name || '')
        : '';
      existingEl.classList.toggle('d-none', !linked);
    }
    document.querySelectorAll('[data-purchase-field]').forEach(function (field) {
      field.classList.toggle('d-none', Boolean(linked));
    });

    applyPurchaseExpiry();

    var dialog = document.getElementById('purchaseItemDialog');
    if (dialog && typeof bootstrap !== 'undefined' && bootstrap.Dialog) {
      bootstrap.Dialog.getOrCreateInstance(dialog).show();
    }
    if (expiryEl && !expiryEl.value) {
      expiryEl.focus();
    }
  }

  function fillCategoryOptions(select, categories) {
    if (!select) return;
    select.innerHTML = '';
    var noneOption = document.createElement('option');
    noneOption.value = '';
    noneOption.textContent = proviant.t("shoppingList.purchaseNoneOption");
    select.appendChild(noneOption);
    categories.forEach(function (category) {
      if (category.hidden) return;
      var option = document.createElement('option');
      option.value = category.categoryKey;
      option.textContent = category.displayName || category.categoryKey;
      select.appendChild(option);
    });
  }

  // The expiry comes from the shelf life the household gave the category. A
  // category without one leaves the date empty and asks for it.
  function applyPurchaseExpiry() {
    var categoryEl = document.getElementById('purchaseCategory');
    var expiryEl = document.getElementById('purchaseExpiry');
    var hint = document.getElementById('purchaseExpiryHint');
    if (!categoryEl || !expiryEl) return;
    var categories = householdCategories();
    var categoryKey = categoryEl.value;
    var days = proviant.categoryShelfLifeDays(categories, categoryKey);
    if (days === null) {
      if (hint) hint.classList.add('d-none');
      if (expiryEl.dataset.userEdited !== '1') {
        expiryEl.value = '';
      }
      return;
    }
    var category = proviant.categoryByKey(categories, categoryKey);
    var applied = proviant.applyExpirySuggestion(expiryEl, categories, categoryKey);
    if (!applied) {
      if (hint) hint.classList.add('d-none');
      return;
    }
    if (hint) {
      hint.textContent = proviant.t("js.shoppingList.expiryFromCategory", category ? category.displayName : categoryKey, days);
      hint.classList.remove('d-none');
    }
  }

  function purchasePayload() {
    var payload = {};
    var quantityEl = document.getElementById('purchaseQuantity');
    if (quantityEl) payload.quantity = Number.parseInt(quantityEl.value, 10) || 1;
    var expiryEl = document.getElementById('purchaseExpiry');
    if (expiryEl && expiryEl.value) payload.expireAt = expiryEl.value;
    var categoryEl = document.getElementById('purchaseCategory');
    // An empty category that the user never touched means the item has no
    // category of its own: the server keeps it. An empty one the user chose
    // means the bought product has no category at all.
    if (categoryEl && (categoryEl.value || categoryTouched)) {
      payload.categoryKey = categoryEl.value;
    }
    var packSizeEl = document.getElementById('purchasePackSize');
    if (packSizeEl) payload.packSize = packSizeEl.value;
    var barcodeEl = document.getElementById('purchaseBarcode');
    if (barcodeEl) payload.barcode = barcodeEl.value;
    var locationEl = document.getElementById('purchaseStorageLocation');
    if (locationEl && locationEl.value) {
      payload.storageLocationId = Number.parseInt(locationEl.value, 10) || null;
    }
    return payload;
  }

  function submitPurchase(extra) {
    if (purchaseItemId === null) return;
    // The item is captured here: the dialog may be closed and opened for
    // another item while the purchase is in flight, and the answer belongs
    // to the item that was bought.
    var itemId = purchaseItemId;
    var payload = purchasePayload();
    if (extra) {
      Object.keys(extra).forEach(function (key) {
        payload[key] = extra[key];
      });
    }
    setPurchaseBusy(true);
    proviant.purchaseShoppingListItem(itemId, payload).then(function (result) {
      setPurchaseBusy(false);
      if (result.code === 409) {
        askAboutDuplicate(result);
        return;
      }
      if (result.code !== 201) {
        proviant.showFeedback('error', proviant.t("js.shoppingList.purchaseFailedTitle"));
        return;
      }
      closePurchaseDialog(itemId);
      markItemChecked(itemId);
      announcePurchase(result.product);
    }).catch(function () {
      setPurchaseBusy(false);
      proviant.showFeedback('error', proviant.t("js.shoppingList.purchaseFailedTitle"));
    });
  }

  // The bought product matches one the household already has. The user
  // decides: the packs belong to that product, or they are a product of
  // their own. The prompt is the one of the product list.
  function askAboutDuplicate(result) {
    var duplicate = result.duplicate;
    if (!duplicate || typeof proviant.showDuplicatePrompt !== 'function') {
      proviant.showFeedback('error', proviant.t("js.shoppingList.purchaseDuplicateTitle"));
      return;
    }
    proviant.showDuplicatePrompt({
      duplicate: duplicate,
      onAddPack: function () {
        submitPurchase({ addToProductId: duplicate.id });
      },
      onAddSeparately: function () {
        submitPurchase({ allowDuplicate: true });
      },
      onCancel: function () {},
    });
  }

  function setPurchaseBusy(busy) {
    var btn = document.getElementById('btnPurchase');
    if (!btn) return;
    btn.disabled = busy;
    btn.classList.toggle('loading', busy);
  }

  // Closes the dialog of the given item. A dialog that shows another item
  // now — the user opened one while the purchase was in flight — stays open.
  function closePurchaseDialog(itemId) {
    if (itemId !== undefined && purchaseItemId !== itemId) return;
    var dialog = document.getElementById('purchaseItemDialog');
    if (dialog && typeof bootstrap !== 'undefined' && bootstrap.Dialog) {
      bootstrap.Dialog.getOrCreateInstance(dialog).hide();
    }
  }

  function markItemChecked(itemId) {
    var row = document.getElementById('item-' + itemId);
    if (row) row.classList.add('checked');
    var checkbox = document.querySelector('.item-checkbox[data-item-id="' + itemId + '"]');
    if (checkbox) checkbox.checked = true;
    updateItemCount();
  }

  function announcePurchase(product) {
    var name = product && product.productName ? product.productName : '';
    var announcement = document.getElementById('purchaseAnnouncement');
    if (announcement) announcement.textContent = proviant.t("js.shoppingList.announcePurchased", name);
    proviant.showFeedback('success', proviant.t("js.shoppingList.purchaseSuccessTitle"), proviant.t("js.shoppingList.purchaseSuccess", name));
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
    var items = shoppingListItems();
    if (!items.length) return;

    // design-tokens:off — the print view is a standalone document with its own
    // inline styles, so it carries the palette values literally.
    var html = '<!DOCTYPE html><html><head><meta charset="utf-8"><title>' + proviant.t("nav.shoppingList") + '</title><style>';
    html += 'body{font-family:system-ui,sans-serif;padding:1rem;color:#2A2A2A}';
    html += 'h1{margin:0 0 0.5rem;font-size:1.5rem}';
    html += '.meta{margin-bottom:1rem;font-size:0.875rem;color:#6B6B6B}';
    html += '.item{display:flex;flex-direction:column;padding:0.5rem 0;border-bottom:1px solid #E5DFD3}';
    html += '.item-row{display:flex;align-items:center}';
    html += '.checkbox{width:18px;height:18px;border:2px solid #2A2A2A;border-radius:3px;margin-right:0.5rem;flex-shrink:0}';
    html += '.name{font-size:1.1rem;flex:1}';
    html += '.qty{font-size:1rem;color:#35604E;margin-left:0.5rem}';
    // design-tokens:on
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
