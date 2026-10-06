// Receipt photo scanning (issue #61): upload, review, bulk import.
// All handlers use event delegation — no DOM references captured at load time.
(function () {
  const MAX_IMAGE_EDGE = 1600;
  // Client-side abort cap derived from the configured server scan timeout
  // (plumbed via window.RECEIPT_SCAN_TIMEOUT_SECONDS from the page template)
  // plus a small grace period, so a longer server timeout never makes the
  // client abort a scan the server is still completing. AbortError surfaces
  // as the generic catch below.
  function receiptScanFetchTimeoutMs() {
    const serverTimeoutSeconds = window.RECEIPT_SCAN_TIMEOUT_SECONDS || 120;
    return serverTimeoutSeconds * 1000 + 500;
  }

  let receiptFile = null;

  function showStep(name) {
    const steps = { upload: "receiptUploadStep", empty: "receiptEmptyStep", review: "receiptReviewStep" };
    Object.entries(steps).forEach(function ([key, elId]) {
      const el = document.getElementById(elId);
      if (el) el.classList.toggle("d-none", key !== name);
    });
  }

  // Screen reader announcements: the progress regions toggle d-none, which is
  // unreliable for live region announcements — state text goes into this
  // persistent (visually hidden) polite region instead.
  function announce(text) {
    const el = document.getElementById("receiptLiveStatus");
    if (el) el.textContent = text;
  }

  // Disable/enable all item card controls while the import request is in
  // flight; removing or editing a card mid-import would desync the UI.
  // collectItems reads .value directly, so disabled inputs are still collected.
  function setReviewDisabled(disabled) {
    document.querySelectorAll("#receiptItems .card").forEach(function (card) {
      card.querySelectorAll("input, select, button").forEach(function (el) {
        el.disabled = disabled;
      });
    });
  }

  // Downscale to ~MAX_IMAGE_EDGE px long edge before upload (design decision:
  // client-side downscale, no new server-side size limit)
  function downscaleImage(file) {
    return new Promise(function (resolve, reject) {
      const img = new Image();
      // CSP allows img-src data: but not blob:, so the file must be loaded
      // as a data URL — an object URL would be blocked and never fire onload.
      const reader = new FileReader();
      img.onload = function () {
        const scale = Math.min(1, MAX_IMAGE_EDGE / Math.max(img.width, img.height));
        const canvas = document.createElement("canvas");
        canvas.width = Math.round(img.width * scale);
        canvas.height = Math.round(img.height * scale);
        canvas.getContext("2d").drawImage(img, 0, 0, canvas.width, canvas.height);
        canvas.toBlob(function (blob) {
          if (blob) resolve(blob);
          else reject(new Error("Could not process image"));
        }, "image/jpeg", 0.85);
      };
      img.onerror = function () {
        reject(new Error("Could not read image"));
      };
      reader.onload = function () {
        img.src = String(reader.result);
      };
      reader.onerror = function () {
        reject(new Error("Could not read image"));
      };
      reader.readAsDataURL(file);
    });
  }

  async function scanReceipt() {
    const btn = document.getElementById("btnScanReceipt");
    const progress = document.getElementById("receiptScanProgress");
    if (!receiptFile) {
      proviant.showFeedback("warning", "No photo", "Please choose or take a receipt photo first.");
      return;
    }
    btn.disabled = true;
    const fileInput = document.getElementById("receiptImage");
    if (fileInput) fileInput.disabled = true;
    progress.classList.remove("d-none");
    announce("Scanning receipt…");
    try {
      const blob = await downscaleImage(receiptFile);
      const formData = new FormData();
      formData.append("image", blob, "receipt.jpg");
      // Hard client-side cap slightly above the server's worst case (config
      // scan timeout + grace period). Without it a stalled server keeps the
      // button disabled forever with no feedback.
      const ctrl = new AbortController();
      const fetchTimeout = setTimeout(function () { ctrl.abort(); }, receiptScanFetchTimeoutMs());
      let res;
      try {
        res = await fetch("/api/v1/products/scan-receipt", { method: "POST", body: formData, signal: ctrl.signal });
      } finally {
        clearTimeout(fetchTimeout);
      }
      const body = await res.json().catch(function () { return {}; });
      if (!res.ok) {
        let message = body.message || "Could not analyze the receipt. Try again later.";
        // Rate limiter answers 429 with Retry-After seconds; a generic
        // "try again later" would send the user straight into the next 429.
        if (res.status === 429) {
          const retryAfter = parseInt(res.headers.get("Retry-After"), 10);
          message = Number.isFinite(retryAfter) && retryAfter > 0
            ? "Too many scans. Please wait " + retryAfter + " second(s), then try again."
            : "Too many scans. Please wait a minute, then try again.";
        }
        proviant.showFeedback("error", "Scan failed", message);
        announce("Scan failed. " + message);
        return;
      }
      const items = Array.isArray(body.items) ? body.items : [];
      if (items.length === 0) {
        showStep("empty");
        announce("No products were recognized on this photo.");
        // showStep hides the upload step while #btnScanReceipt still holds
        // focus — without this, keyboard focus silently drops to <body>.
        const retryBtn = document.getElementById("btnRetryEmpty");
        if (retryBtn) retryBtn.focus();
        return;
      }
      renderReview(items);
      showStep("review");
      announce(items.length + " product(s) found. Review the list and confirm.");
      const firstNameInput = document.querySelector("#receiptItems .card .receipt-item-name");
      if (firstNameInput) firstNameInput.focus();
    } catch (err) {
      let message = err.message || "Could not connect to the server.";
      if (err.name === "AbortError") {
        message = "The scan took too long and was cancelled. Try again.";
      }
      proviant.showFeedback("error", "Scan failed", message);
      announce("Scan failed. " + message);
    } finally {
      btn.disabled = false;
      if (fileInput) fileInput.disabled = false;
      progress.classList.add("d-none");
    }
  }

  function renderReview(items) {
    const container = document.getElementById("receiptItems");
    container.innerHTML = "";
    items.forEach(function (item, index) {
      container.appendChild(renderItemCard(item, index));
    });
    updateImportCount();
  }

  function renderItemCard(item, index) {
    const card = document.createElement("div");
    card.className = "card";
    card.dataset.index = String(index);
    card.innerHTML =
      '<div class="card-body py-2">' +
      '<div class="d-flex align-items-start gap-2">' +
      '<input type="text" class="form-control form-control-sm receipt-item-name" value="" aria-label="Product name" placeholder="Product name">' +
      '<button type="button" class="btn btn-sm btn-proviant-secondary flex-shrink-0" data-receipt-remove aria-label="Remove item">' +
      '<i class="bi bi-x-lg" aria-hidden="true"></i></button>' +
      "</div>" +
      '<div class="d-flex gap-2 mt-2">' +
      '<input type="number" class="form-control form-control-sm receipt-item-amount" min="1" max="999" aria-label="Amount">' +
      '<input type="text" class="form-control form-control-sm receipt-item-unit" value="" aria-label="Unit" placeholder="Unit">' +
      '<input type="number" class="form-control form-control-sm receipt-item-price" min="0" max="100000" step="0.01" aria-label="Price" placeholder="Price">' +
      "</div>" +
      '<div class="row g-2 mt-1">' +
      '<div class="col-4"><input type="date" class="form-control form-control-sm receipt-item-expire" aria-label="Expiry date override"></div>' +
      '<div class="col-4"><input type="text" class="form-control form-control-sm receipt-item-category" aria-label="Category override" placeholder="Category"></div>' +
      '<div class="col-4"><select class="form-select form-select-sm receipt-item-location" aria-label="Storage location override"></select></div>' +
      "</div>" +
      "</div>";
    fillLocationOptions(card.querySelector(".receipt-item-location"));
    card.querySelector(".receipt-item-name").value = item.name || "";
    card.querySelector(".receipt-item-amount").value = item.amount || 1;
    card.querySelector(".receipt-item-unit").value = item.unit || "";
    card.querySelector(".receipt-item-price").value = item.price != null ? item.price : "";
    return card;
  }

  // Options are built via DOM APIs so option values (server-rendered IDs)
  // never pass through an HTML attribute string unescaped.
  function fillLocationOptions(select) {
    if (!select) return;
    const batchSelect = document.getElementById("batchStorageLocation");
    function addOption(value, text) {
      const opt = document.createElement("option");
      opt.value = value;
      opt.textContent = text;
      select.appendChild(opt);
    }
    addOption("", "— default —");
    if (!batchSelect) return;
    Array.prototype.forEach.call(batchSelect.options, function (opt) {
      if (opt.value === "") return;
      addOption(opt.value, opt.textContent);
    });
  }

  // Mirror the server's truncateUTF8: cut to max UTF-8 bytes without
  // splitting a multi-byte rune (server clamps/rejects by byte length, so a
  // char-based slice would let a multibyte name through only to fail there).
  function truncateUtf8Bytes(str, max) {
    const bytes = new TextEncoder().encode(str);
    if (bytes.length <= max) return str;
    let cut = max;
    while (cut > 0 && (bytes[cut] & 0xc0) === 0x80) cut--;
    return new TextDecoder().decode(bytes.slice(0, cut));
  }

  function collectItems() {
    const items = [];
    const batchExpire = (document.getElementById("batchExpireAt") || {}).value || "";
    const batchLocation = (document.getElementById("batchStorageLocation") || {}).value || "";
    const batchCategory = ((document.getElementById("batchCategories") || {}).value || "").trim();
    let invalid = 0;

    document.querySelectorAll("#receiptItems .card").forEach(function (card) {
      const rawName = card.querySelector(".receipt-item-name").value.trim();
      if (!rawName) { invalid++; return; }
      // Mirror the server clamps (util.ReceiptItemMaxNameLength / ReceiptItemMaxAmount)
      // so the value the user confirms is the value that gets stored.
      const name = truncateUtf8Bytes(rawName, 200);
      const amount = Math.min(999, Math.max(1, parseInt(card.querySelector(".receipt-item-amount").value, 10) || 1));
      const unit = truncateUtf8Bytes(card.querySelector(".receipt-item-unit").value.trim(), 20);
      const priceRaw = parseFloat(card.querySelector(".receipt-item-price").value);
      const expire = card.querySelector(".receipt-item-expire").value || batchExpire;
      const category = truncateUtf8Bytes(card.querySelector(".receipt-item-category").value.trim() || batchCategory, 100);
      const location = card.querySelector(".receipt-item-location").value || batchLocation;
      const draft = {
        productName: name,
        amount: amount,
        unit: unit,
        expireAt: expire,
        categories: category,
        storageLocationId: location ? parseInt(location, 10) : null,
        isPrivate: false,
      };
      if (!isNaN(priceRaw) && priceRaw > 0) draft.priceOverride = Math.min(priceRaw, 100000);
      items.push(draft);
    });
    return { items: items, invalid: invalid };
  }

  function updateImportCount() {
    const el = document.getElementById("receiptImportCount");
    if (el) el.textContent = String(collectItems().items.length);
  }

  async function importItems() {
    const btn = document.getElementById("btnImportReceipt");
    const progress = document.getElementById("receiptImportProgress");
    const collected = collectItems();
    if (collected.items.length === 0) {
      proviant.showFeedback("warning", "Nothing to add", collected.invalid > 0
        ? "Some items have no product name. Fill in a name or remove them."
        : "Add at least one product.");
      return;
    }
    // Items without a product name are skipped; the count is folded into the
    // final import-result feedback instead of a separate pre-flight modal,
    // which fast imports would immediately replace.
    const skipped = collected.invalid;
    const skippedNote = skipped > 0 ? " " + skipped + " item(s) without a product name were skipped." : "";
    const maxItems = window.RECEIPT_SCAN_MAX_ITEMS || 100;
    if (collected.items.length > maxItems) {
      proviant.showFeedback("warning", "Too many items", "Only the first " + maxItems + " item(s) will be added.");
      collected.items = collected.items.slice(0, maxItems);
    }
    btn.disabled = true;
    const rescanBtn = document.getElementById("btnRescanReceipt");
    if (rescanBtn) rescanBtn.disabled = true;
    setReviewDisabled(true);
    progress.classList.remove("d-none");
    announce("Adding " + collected.items.length + " product(s)…");
    // Success and partial paths hand control to a redirect callback, so the
    // controls must stay disabled there — re-enabling would let the import be
    // triggered twice before navigation happens.
    let importSettled = false;
    try {
      // 100 items insert locally in well under a second; the cap only bounds
      // a stalled or dead server so the button is not disabled forever.
      const ctrl = new AbortController();
      const fetchTimeout = setTimeout(function () { ctrl.abort(); }, 30000);
      let res;
      try {
        res = await fetch("/api/v1/products/bulk", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ items: collected.items }),
          signal: ctrl.signal,
        });
      } finally {
        clearTimeout(fetchTimeout);
      }
      const body = await res.json().catch(function () { return {}; });
      if (!res.ok) {
        proviant.showFeedback("error", "Import failed", body.message || "Could not add the products.");
        announce("Import failed. " + (body.message || "Could not add the products."));
        return;
      }
      const results = Array.isArray(body.results) ? body.results : [];
      const created = results.filter(function (r) { return r.status === "created"; }).length;
      const failed = results.length - created;
      if (failed === 0) {
        importSettled = true;
        announce(created + " product(s) added from the receipt." + skippedNote);
        proviant.showFeedback("success", "Products added", created + " product(s) added from the receipt." + skippedNote, function () {
          window.location.href = "/web/products";
        });
      } else {
        importSettled = true;
        announce(created + " product(s) added, " + failed + " failed." + skippedNote + " Check the products list.");
        proviant.showFeedback("warning", "Partially added", created + " product(s) added, " + failed + " failed." + skippedNote + " Check the products list.", function () {
          window.location.href = "/web/products";
        });
      }
    } catch (err) {
      let message = err.message || "Could not connect to the server.";
      if (err.name === "AbortError") {
        // The server may still have inserted some products before the client
        // gave up — retrying blindly can create duplicates.
        message = "Adding the products took too long and was cancelled. The import may have partially completed — check the products list before retrying.";
      }
      proviant.showFeedback("error", "Import failed", message);
      announce("Import failed. " + message);
    } finally {
      if (!importSettled) {
        btn.disabled = false;
        if (rescanBtn) rescanBtn.disabled = false;
        setReviewDisabled(false);
      }
      progress.classList.add("d-none");
    }
  }

  function resetToUpload() {
    receiptFile = null;
    const input = document.getElementById("receiptImage");
    if (input) input.value = "";
    showStep("upload");
  }

  document.addEventListener("change", function (event) {
    if (event.target.closest("#receiptImage")) {
      receiptFile = event.target.files && event.target.files[0] ? event.target.files[0] : null;
    }
  });

  document.addEventListener("input", function (event) {
    if (event.target.closest("#receiptItems")) {
      updateImportCount();
    }
  });

  document.addEventListener("click", function (event) {
    const scanBtn = event.target.closest("#btnScanReceipt");
    if (scanBtn) {
      event.preventDefault();
      scanReceipt();
      return;
    }
    const removeBtn = event.target.closest("[data-receipt-remove]");
    if (removeBtn) {
      event.preventDefault();
      const card = removeBtn.closest(".card");
      if (card) {
        // Removing the focused control would drop keyboard focus to <body>
        // (WCAG 2.4.3): park focus on a sibling card first, or the empty
        // step's retry button when this was the last card.
        const focusLost = card.contains(document.activeElement);
        const cards = Array.prototype.slice.call(document.querySelectorAll("#receiptItems .card"));
        card.remove();
        const remaining = Array.prototype.slice.call(document.querySelectorAll("#receiptItems .card"));
        if (remaining.length === 0) {
          showStep("empty");
          announce("All items removed.");
          if (focusLost) {
            const retryBtn = document.getElementById("btnRetryEmpty");
            if (retryBtn) retryBtn.focus();
          }
        } else if (focusLost) {
          const nextCard = remaining[Math.min(cards.indexOf(card), remaining.length - 1)] || remaining[remaining.length - 1];
          const nameInput = nextCard.querySelector(".receipt-item-name");
          if (nameInput) nameInput.focus();
        }
      }
      updateImportCount();
      return;
    }
    if (event.target.closest("#btnImportReceipt")) {
      event.preventDefault();
      importItems();
      return;
    }
    if (event.target.closest("#btnRetryEmpty") || event.target.closest("#btnRescanReceipt")) {
      event.preventDefault();
      // Review items (scanned + user edits) are lost on rescan; confirm
      // before discarding, but only when there is something to lose.
      const itemCount = document.querySelectorAll("#receiptItems .card").length;
      if (itemCount > 0) {
        proviant.showConfirm(
          "Discard scanned products",
          "Discard the " + itemCount + " scanned product(s)?",
          function() { resetToUpload(); },
          "Discard",
          "danger"
        );
        return;
      }
      resetToUpload();
    }
  });
})();
