/* Household category management: the display name, the order, the visibility
   and the shelf-life defaults of the household's categories. Every write goes
   to the API — the list on the page is the household's, not the browser's. */
(function () {
  'use strict';

  function categoryRows() {
    return Array.from(document.querySelectorAll('#householdCategoryList li[data-category-key]'));
  }

  function rowValues(row) {
    const nameInput = row.querySelector('.cat-name');
    const shelfLifeInput = row.querySelector('.cat-shelf-life');
    const daysAfterOpeningInput = row.querySelector('.cat-days-after-opening');
    const visibleInput = row.querySelector('.cat-visible');
    return {
      displayName: nameInput ? nameInput.value.trim() : '',
      defaultShelfLifeDays: shelfLifeInput && shelfLifeInput.value !== ''
        ? Number.parseInt(shelfLifeInput.value, 10)
        : 0,
      defaultDaysAfterOpening: daysAfterOpeningInput && daysAfterOpeningInput.value !== ''
        ? Number.parseInt(daysAfterOpeningInput.value, 10)
        : 0,
      hidden: visibleInput ? !visibleInput.checked : false,
    };
  }

  function feedback(type, title, message) {
    if (typeof proviant.showFeedback !== 'function') return;
    proviant.showFeedback(type, title, message);
  }

  async function saveCategory(row) {
    const categoryKey = row.dataset.categoryKey;
    if (!categoryKey) return;
    const button = row.querySelector('.btn-cat-save');
    const values = rowValues(row);
    if (button) button.disabled = true;
    try {
      const response = await fetch(`/api/v1/household/categories/${encodeURIComponent(categoryKey)}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          displayName: values.displayName,
          hidden: values.hidden,
          defaultShelfLifeDays: values.defaultShelfLifeDays,
          defaultDaysAfterOpening: values.defaultDaysAfterOpening,
        }),
      });
      const body = await response.json().catch(function () { return {}; });
      if (!response.ok) {
        feedback('error', proviant.t('common.error'), body.message || proviant.t('js.settings.unknownError'));
        return;
      }
      feedback('success', proviant.t('js.settings.categorySaved'), proviant.t('js.settings.categorySavedMessage', values.displayName));
    } catch (error) {
      feedback('error', proviant.t('common.error'), error && error.message ? error.message : String(error));
    } finally {
      if (button) button.disabled = false;
    }
  }

  // Moves a category one place up or down and writes the whole order, so the
  // list on the page and the order in the database stay the same. A write that
  // fails puts the row back where it was: the list on the page has to show the
  // order the database holds.
  async function moveCategory(row, offset) {
    const list = document.getElementById('householdCategoryList');
    if (!list || !row) return;
    const rows = categoryRows();
    const index = rows.indexOf(row);
    const target = index + offset;
    if (index < 0 || target < 0 || target >= rows.length) return;

    const neighbour = rows[target];
    if (offset < 0) list.insertBefore(row, neighbour);
    else list.insertBefore(neighbour, row);

    const keys = categoryRows().map(function (entry) {
      return entry.dataset.categoryKey;
    });
    try {
      const response = await fetch('/api/v1/household/categories/order', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ keys: keys }),
      });
      const body = await response.json().catch(function () { return {}; });
      if (!response.ok) {
        feedback('error', proviant.t('common.error'), body.message || proviant.t('js.settings.unknownError'));
        restoreOrder(list, row, neighbour, offset);
        return;
      }
    } catch (error) {
      feedback('error', proviant.t('common.error'), error && error.message ? error.message : String(error));
      restoreOrder(list, row, neighbour, offset);
    }
  }

  // Puts a moved row back in the place it came from.
  function restoreOrder(list, row, neighbour, offset) {
    if (offset < 0) list.insertBefore(neighbour, row);
    else list.insertBefore(row, neighbour);
  }

  document.addEventListener('click', function (event) {
    const saveButton = event.target.closest('.btn-cat-save');
    if (saveButton) {
      event.preventDefault();
      const row = saveButton.closest('li[data-category-key]');
      void saveCategory(row);
      return;
    }

    const upButton = event.target.closest('.btn-cat-up');
    if (upButton) {
      event.preventDefault();
      void moveCategory(upButton.closest('li[data-category-key]'), -1);
      return;
    }

    const downButton = event.target.closest('.btn-cat-down');
    if (downButton) {
      event.preventDefault();
      void moveCategory(downButton.closest('li[data-category-key]'), 1);
    }
  });
})();
