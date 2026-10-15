/* CSV import and JSON backup restore for the products page.
 *
 * Every listener is delegated off `document`: this script is loaded on a page
 * that also renders the mobile and desktop menus from templates, and a captured
 * element reference goes stale after a reload or a DOM swap.
 */

proviant.importProductsCSV = async function (file) {
  const body = new FormData();
  body.append("file", file, file.name);
  // Content-Type is deliberately unset: the browser must add the multipart
  // boundary itself. The global fetch interceptor still adds X-CSRF-Token.
  const res = await fetch("/api/v1/products/import", { method: "POST", body: body });
  // A proxy answering 413/502 with an HTML body would make res.json() throw;
  // the request still reached *something*, so keep its status as the message.
  let result;
  try {
    result = await res.json();
  } catch (_parseErr) {
    result = { message: res.statusText || "The CSV could not be imported." };
  }
  return { code: res.status, message: result };
};

proviant.downloadImportTemplate = function () {
  window.location.href = `${globalThis.location.protocol}//${globalThis.location.host}/api/v1/products/import/template.csv`;
};

proviant.restoreBackupJSON = async function (file) {
  const body = new FormData();
  body.append("file", file, file.name);
  // Content-Type is deliberately unset: the browser must add the multipart
  // boundary itself. The global fetch interceptor still adds X-CSRF-Token.
  const res = await fetch("/api/v1/products/restore/full.json", { method: "POST", body: body });
  // A proxy answering 413/502 with an HTML body would make res.json() throw;
  // keep the transport status as the message in that case.
  let result;
  try {
    result = await res.json();
  } catch (_parseErr) {
    result = { message: res.statusText || "The backup could not be restored." };
  }
  return { code: res.status, message: result };
};

document.addEventListener("DOMContentLoaded", function () {
  const fileInput = document.getElementById("csvImportFile");
  const restoreInput = document.getElementById("jsonRestoreFile");
  let restoreTrigger = null;

  function importCSVFile(file) {
    if (!file) return;
    proviant.importProductsCSV(file)
      .then(function (result) {
        const data = result.message;
        if (result.code !== 200) {
          proviant.showFeedback("error", "Import failed", importFailureText(data));
          return;
        }
        proviant.showFeedback("success", "Import complete", importSummary(data), function () {
          globalThis.location.reload();
        });
      })
      .catch(function () {
        proviant.showFeedback("error", "Import failed", "Could not reach the server.");
      });
  }

  // showFeedback renders a single line, so only the first few reasons fit.
  function importReasons(data) {
    if (!data.errors || !data.errors.length) return "";
    return " " + data.errors.slice(0, 3).map(function (e) {
      return `Row ${e.row}: ${e.reason}`;
    }).join(" · ");
  }

  function importSummary(data) {
    let summary = `${data.imported} imported, ${data.failed} failed of ${data.totalRows} rows.`;
    if (data.createdLocations && data.createdLocations.length) {
      summary += ` Created storage locations: ${data.createdLocations.join(", ")}.`;
    }
    return summary + importReasons(data);
  }

  // A request-level rejection (no file part, no barcode column) answers with a
  // bare message; a file that parsed but had nothing importable answers with the
  // same body as a success, so its per-row reasons are worth showing.
  function importFailureText(data) {
    if (data && data.errors && data.errors.length) {
      return importSummary(data);
    }
    return (data && data.message) || "The CSV could not be imported.";
  }

  // showFeedback renders a single line, so the restore summary stays short.
  function restoreSummary(data) {
    let summary = `${data.productsImported} active and ${data.productsArchived} archived products restored.`;
    if (data.locationsCreated) {
      summary += ` ${data.locationsCreated} storage locations created.`;
    }
    return summary;
  }

  function restoreFailureText(data) {
    return (data && data.message) || "The backup could not be restored.";
  }

  function setRestoreBusy(trigger, busy) {
    if (!trigger) return;
    trigger.disabled = busy;
    trigger.setAttribute("aria-disabled", busy ? "true" : "false");
    if (busy) {
      trigger.dataset.restoreOriginalHtml = trigger.innerHTML;
      trigger.innerHTML = '<i class="bi bi-hourglass-split me-2"></i>Restoring…';
      return;
    }
    trigger.innerHTML = trigger.dataset.restoreOriginalHtml || trigger.innerHTML;
    delete trigger.dataset.restoreOriginalHtml;
  }

  function restoreBackupFile(file, trigger) {
    if (!file) return;
    proviant.showConfirm(
      "Restore backup?",
      `"${file.name}" will replace this household's products and storage locations, overwrite household settings, and unlink shopping list, recipe and scan history entries from their products. This cannot be undone.`,
      function () {
        setRestoreBusy(trigger, true);
        proviant.restoreBackupJSON(file)
          .then(function (result) {
            const data = result.message;
            if (result.code !== 200) {
              proviant.showFeedback("error", "Restore failed", restoreFailureText(data));
              return;
            }
            proviant.showFeedback("success", "Restore complete", restoreSummary(data), function () {
              globalThis.location.reload();
            });
          })
          .catch(function () {
            proviant.showFeedback("error", "Restore failed", "Could not reach the server.");
          })
          .finally(function () {
            setRestoreBusy(trigger, false);
          });
      },
      "Restore and replace",
      "danger"
    );
  }

  if (fileInput) {
    fileInput.addEventListener("change", function () {
      const file = fileInput.files && fileInput.files[0];
      // Reset first so picking the same file twice still fires a change event.
      fileInput.value = "";
      importCSVFile(file);
    });
  }

  if (restoreInput) {
    restoreInput.addEventListener("change", function () {
      const file = restoreInput.files && restoreInput.files[0];
      // Reset first so picking the same file twice still fires a change event.
      restoreInput.value = "";
      restoreBackupFile(file, restoreTrigger);
    });
  }

  document.addEventListener("click", function (event) {
    const restoreButton = event.target.closest("#btnRestoreBackupJSON, #mobile-restore-json, #btnRestoreBackupJSONEmpty");
    if (!restoreButton) return;
    event.preventDefault();
    if (restoreButton.disabled) return;
    restoreTrigger = restoreButton;
    const input = document.getElementById("jsonRestoreFile");
    if (input) input.click();
  });

  document.addEventListener("click", function (event) {
    if (!event.target.closest("#btnImportProductsCSV, #mobile-import-csv, #btnImportProductsCSVEmpty")) return;
    event.preventDefault();
    const input = document.getElementById("csvImportFile");
    if (input) input.click();
  });

  document.addEventListener("click", function (event) {
    if (!event.target.closest("#btnDownloadImportTemplate, #mobile-import-template")) return;
    event.preventDefault();
    proviant.downloadImportTemplate();
  });
});
