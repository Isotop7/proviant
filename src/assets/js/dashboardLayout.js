(function () {
  'use strict';

  const LAYOUT_VERSION = 1;
  const STORAGE_KEY = 'proviant_dashboard_layout';
  const DESKTOP_COLUMNS = 6;
  const TABLET_COLUMNS = 4;
  const PHONE_COLUMNS = 2;
  const MAX_COLUMNS = 6;
  const MAX_ROWS = 12;
  const SAVE_DEBOUNCE_MS = 300;
  const SLOT_EXIT_MS = 120;

  // The width of a tile is stored in columns of the desktop grid. The narrower
  // grids derive their spans from it, so a dashboard nobody touched keeps the
  // spans the design system's screen sheet gives it. The height is stored in
  // rows of the grid; a tile that needs more room than one row asks for it.
  const DEFAULT_LAYOUT = {
    'hero': { order: 0, w: 3, h: 1 },
    'waste-rate': { order: 1, w: 3, h: 1 },
    'consumed': { order: 2, w: 2, h: 1 },
    'last-added': { order: 3, w: 2, h: 1 },
    'streak': { order: 4, w: 2, h: 1 },
    'goal': { order: 5, w: 2, h: 1 },
    'activity': { order: 6, w: 6, h: 2 },
    'savings-eur': { order: 7, w: 2, h: 1 },
    'savings-co2': { order: 8, w: 2, h: 1 },
    'expiring-list': { order: 9, w: 6, h: 2 },
    'chart-waste': { order: 10, w: 3, h: 2 },
    'chart-categories': { order: 11, w: 3, h: 2 },
    'chart-expiry': { order: 12, w: 6, h: 2 },
    'cta-waste': { order: 13, w: 3, h: 1 },
    'cta-recipes': { order: 14, w: 3, h: 1 },
  };

  let grid = null;
  let tiles = [];
  let storedLayout = null;
  let drag = null;
  let resize = null;
  let saveTimer = null;
  let editing = false;
  let slot = null;
  let slotTimer = null;

  function announce(message) {
    const status = document.getElementById('dashboard-status');
    if (status) status.textContent = message;
  }

  function clamp(value, min, max) {
    return Math.min(max, Math.max(min, value));
  }

  // The number of columns the grid has at the current width.
  function currentColumns() {
    if (!window.matchMedia) return DESKTOP_COLUMNS;
    if (window.matchMedia('(max-width: 575.98px)').matches) return PHONE_COLUMNS;
    if (window.matchMedia('(max-width: 991.98px)').matches) return TABLET_COLUMNS;
    return DESKTOP_COLUMNS;
  }

  // A tile that covers half the desktop grid or more covers the whole tablet
  // grid, and the phone grid stacks the tiles.
  function spanFor(width, columns) {
    if (columns >= DESKTOP_COLUMNS) return clamp(width, 1, columns);
    if (columns === TABLET_COLUMNS) return width >= 3 ? TABLET_COLUMNS : width;
    return width >= 2 ? PHONE_COLUMNS : 1;
  }

  function tileLabel(element) {
    if (element.dataset.tileLabel) return element.dataset.tileLabel;
    const heading = element.querySelector('.kpi__label, .chart-card__eyebrow, .card-header');
    if (!heading) return '';
    return heading.textContent.replace(/\s+/g, ' ').trim();
  }

  function tileFor(element) {
    for (let i = 0; i < tiles.length; i++) {
      if (tiles[i].element === element) return tiles[i];
    }
    return null;
  }

  function orderedTiles() {
    return tiles.slice().sort(function (a, b) { return a.order - b.order; });
  }

  function defaultEntry(id, fallbackOrder) {
    const defaults = DEFAULT_LAYOUT[id];
    if (defaults) return { order: defaults.order, w: defaults.w, h: defaults.h };
    return { order: fallbackOrder, w: 2, h: 1 };
  }

  // Collects the tiles from the grid and gives each of them its position, its
  // width and its height: the stored values when the user arranged the
  // dashboard, the defaults otherwise.
  function readTiles() {
    const elements = Array.prototype.slice.call(grid.querySelectorAll('[data-tile]'));
    const stored = storedLayout && storedLayout.tiles ? storedLayout.tiles : {};

    tiles = elements.map(function (element, index) {
      const id = element.dataset.tile;
      const defaults = defaultEntry(id, Object.keys(DEFAULT_LAYOUT).length + index);
      const saved = stored[id];
      return {
        id: id,
        element: element,
        label: tileLabel(element),
        order: saved && typeof saved.order === 'number' ? saved.order : defaults.order,
        w: saved && typeof saved.w === 'number' ? clamp(saved.w, 1, MAX_COLUMNS) : defaults.w,
        h: saved && typeof saved.h === 'number' ? clamp(saved.h, 1, MAX_ROWS) : defaults.h,
      };
    });

    tiles.sort(function (a, b) { return a.order - b.order; });
    tiles.forEach(function (tile, position) { tile.order = position; });
  }

  // Writes the arrangement onto the grid: the position of a tile, the columns
  // it spans and the rows it spans.
  function applyLayout() {
    const columns = currentColumns();
    tiles.forEach(function (tile) {
      tile.element.style.setProperty('--tile-order', String(tile.order));
      tile.element.style.setProperty('--tile-w', String(spanFor(tile.w, columns)));
      tile.element.style.setProperty('--tile-h', String(tile.h));
      tile.element.dataset.tileSpan = 'true';
    });
  }

  // Moves a tile to a position in the arrangement.
  function moveTileTo(tile, index) {
    const ordered = orderedTiles();
    const current = ordered.indexOf(tile);
    const target = clamp(index, 0, ordered.length - 1);
    if (current === target) return false;

    // The dragged tile follows the pointer, so it must not be animated.
    animateReflow(drag ? drag.tile : null, function () {
      ordered.splice(current, 1);
      ordered.splice(target, 0, tile);
      ordered.forEach(function (entry, position) { entry.order = position; });
      applyLayout();
    });
    return true;
  }

  function layoutPayload() {
    const payload = { version: LAYOUT_VERSION, tiles: {} };
    tiles.forEach(function (tile) {
      payload.tiles[tile.id] = { order: tile.order, w: tile.w, h: tile.h };
    });
    return payload;
  }

  function writeLocalLayout(payload) {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(payload));
    } catch (error) {
      void error;
    }
  }

  function readLocalLayout() {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return null;
      return JSON.parse(raw);
    } catch (error) {
      void error;
      return null;
    }
  }

  function clearLocalLayout() {
    try {
      localStorage.removeItem(STORAGE_KEY);
    } catch (error) {
      void error;
    }
  }

  // Stores the arrangement on the server. The local copy keeps the dashboard
  // in shape while the request is on its way and survives an offline save.
  function saveLayout(payload) {
    writeLocalLayout(payload);
    if (saveTimer) {
      clearTimeout(saveTimer);
      saveTimer = null;
    }
    if (typeof proviant.saveDashboardLayout !== 'function') return;

    proviant.saveDashboardLayout(payload).then(function (response) {
      if (response.code === 200) return;
      proviant.showFeedback('error', proviant.t("js.home.layoutSaveFailed"), response.message || proviant.t("js.home.layoutSaveFailed"));
    }).catch(function () {
      proviant.showFeedback('error', proviant.t("js.home.layoutSaveFailed"), proviant.t("js.home.layoutSaveFailed"));
    });
  }

  function scheduleSave() {
    const payload = layoutPayload();
    writeLocalLayout(payload);
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(function () {
      saveTimer = null;
      saveLayout(payload);
    }, SAVE_DEBOUNCE_MS);
  }

  function setResetEnabled(enabled) {
    const button = document.getElementById('dashboard-reset-layout');
    if (button) button.disabled = !enabled;
  }

  // Writes the edit mode into the dashboard: the grips, the reset button and
  // the label of the toggle.
  function applyEditMode() {
    if (!grid) return;

    grid.classList.toggle('bento--editing', editing);

    const toggle = document.getElementById('dashboard-edit-layout');
    if (toggle) {
      const label = toggle.querySelector('.dashboard-tools__label');
      if (label) label.textContent = editing ? toggle.dataset.doneLabel : toggle.dataset.editLabel;
    }

    const reset = document.getElementById('dashboard-reset-layout');
    if (reset) reset.hidden = !editing;

    updateHandleVisibility();
  }

  // The grips and the reset button belong to the edit mode: they only show
  // while the dashboard is being arranged.
  function setEditing(enabled) {
    if (!grid) return;
    const next = Boolean(enabled);
    if (editing === next) return;
    editing = next;

    applyEditMode();

    // The controls that just vanished must not keep the focus.
    if (!editing) {
      const active = document.activeElement;
      const reset = document.getElementById('dashboard-reset-layout');
      const onVanishedControl = active && (
        (active.closest && active.closest('.bento__handle') && grid.contains(active)) ||
        (reset && active === reset)
      );
      if (onVanishedControl) {
        const toggle = document.getElementById('dashboard-edit-layout');
        if (toggle) toggle.focus();
      }
    }

    announce(proviant.t(editing ? "js.home.editModeOn" : "js.home.editModeOff"));
  }

  function hasStoredArrangement() {
    if (!storedLayout || !storedLayout.tiles) return false;
    return Object.keys(storedLayout.tiles).some(function (id) {
      return tiles.some(function (tile) { return tile.id === id; });
    });
  }

  function buildHandle(kind, tile) {
    const handle = document.createElement('button');
    handle.type = 'button';
    handle.className = 'bento__handle bento__handle--' + kind;
    handle.dataset.tileHandle = kind;
    const icon = document.createElement('i');
    icon.className = kind === 'move' ? 'bi bi-grip-vertical' : 'bi bi-arrows-angle-expand';
    icon.setAttribute('aria-hidden', 'true');
    handle.appendChild(icon);
    const label = tile.label || tile.id;
    handle.setAttribute('aria-label', kind === 'move'
      ? proviant.t("js.home.moveTile", label)
      : proviant.t("js.home.resizeTile", label));
    handle.setAttribute('title', handle.getAttribute('aria-label'));
    return handle;
  }

  // The grips only show in the edit mode, and the resize grip only where the
  // grid has the columns it resizes.
  function updateHandleVisibility() {
    const resizable = currentColumns() >= DESKTOP_COLUMNS;
    tiles.forEach(function (tile) {
      const move = tile.element.querySelector('.bento__handle--move');
      const resize = tile.element.querySelector('.bento__handle--resize');
      if (move) move.hidden = !editing;
      if (resize) resize.hidden = !editing || !resizable;
    });
  }

  function attachHandles() {
    tiles.forEach(function (tile) {
      if (tile.element.querySelector('.bento__handle')) return;
      tile.element.appendChild(buildHandle('move', tile));
      tile.element.appendChild(buildHandle('resize', tile));
    });
    updateHandleVisibility();
  }

  // The landing shadow: it marks the place the dragged tile will take and the
  // size it will have there.
  function ensureSlot() {
    if (slot) return slot;
    slot = document.createElement('div');
    slot.className = 'bento__slot';
    slot.setAttribute('aria-hidden', 'true');
    grid.prepend(slot);
    return slot;
  }

  // The slot follows the cell without the drag offset, so it stays where the
  // tile will land.
  function positionSlot(cell) {
    if (!slot) return;
    const rect = cell.getBoundingClientRect();
    const gridRect = grid.getBoundingClientRect();
    slot.style.left = (rect.left - drag.transformX - gridRect.left) + 'px';
    slot.style.top = (rect.top - drag.transformY - gridRect.top) + 'px';
    slot.style.width = rect.width + 'px';
    slot.style.height = rect.height + 'px';
  }

  function showSlot(cell) {
    ensureSlot();
    positionSlot(cell);
    // The slot has to be in the layout before the class changes, otherwise the
    // fade in has no starting point.
    void slot.offsetWidth;
    slot.classList.add('bento__slot--visible');
  }

  function hideSlot() {
    if (!slot) return;
    slot.classList.remove('bento__slot--visible');
    if (slotTimer) clearTimeout(slotTimer);
    slotTimer = setTimeout(function () {
      slotTimer = null;
      if (!slot) return;
      slot.remove();
      slot = null;
    }, SLOT_EXIT_MS);
  }

  // The length of a transition in the current theme. The reduce-motion theme
  // sets the motion tokens to zero, which switches the animations off.
  function motionDuration() {
    const styles = window.getComputedStyle(document.documentElement);
    const duration = parseFloat(styles.getPropertyValue('--motion-base'));
    return isNaN(duration) ? 0 : duration;
  }

  // The tiles that change their place glide there: the grid has already placed
  // them, the animation plays the difference back.
  function animateReflow(excluded, mutate) {
    const moved = tiles.filter(function (tile) { return tile.element !== excluded; });
    if (moved.length === 0 || typeof moved[0].element.animate !== 'function') {
      mutate();
      return;
    }

    const before = moved.map(function (tile) {
      return { element: tile.element, rect: tile.element.getBoundingClientRect() };
    });

    mutate();

    const duration = motionDuration();
    if (!duration) return;

    before.forEach(function (entry) {
      const next = entry.element.getBoundingClientRect();
      const dx = entry.rect.left - next.left;
      const dy = entry.rect.top - next.top;
      if (!dx && !dy) return;
      entry.element.animate(
        [{ transform: 'translate(' + dx + 'px, ' + dy + 'px)' }, { transform: 'translate(0, 0)' }],
        { duration: duration, easing: 'cubic-bezier(0.23, 1, 0.32, 1)' }
      );
    });
  }

  // The insertion point of a pointer: the number of tiles whose centre lies
  // before it in reading order.
  function insertionIndex(x, y, dragged) {
    const others = orderedTiles().filter(function (tile) { return tile !== dragged; });
    let index = 0;
    for (let i = 0; i < others.length; i++) {
      const rect = others[i].element.getBoundingClientRect();
      const centerX = rect.left + rect.width / 2;
      const centerY = rect.top + rect.height / 2;
      const band = rect.height / 4;
      if (y > centerY + band) {
        index++;
        continue;
      }
      if (y < centerY - band) break;
      if (x > centerX) index++;
      else break;
    }
    return index;
  }

  function onMovePointerDown(event, handle) {
    const cell = handle.closest('[data-tile]');
    if (!cell) return;
    const tile = tileFor(cell);
    if (!tile) return;

    const rect = cell.getBoundingClientRect();
    drag = {
      tile: tile,
      pointerId: event.pointerId,
      grabOffsetX: event.clientX - rect.left,
      grabOffsetY: event.clientY - rect.top,
      transformX: 0,
      transformY: 0,
      index: orderedTiles().indexOf(tile),
    };

    if (handle.setPointerCapture) handle.setPointerCapture(event.pointerId);
    cell.classList.remove('bento__cell--landed');
    cell.classList.add('bento__cell--dragging');
    showSlot(cell);
    document.addEventListener('pointermove', onMovePointerMove);
    document.addEventListener('pointerup', onMovePointerUp);
    document.addEventListener('pointercancel', onMovePointerUp);
    event.preventDefault();
  }

  function onMovePointerMove(event) {
    if (!drag) return;
    const cell = drag.tile.element;

    const rect = cell.getBoundingClientRect();
    const slotLeft = rect.left - drag.transformX;
    const slotTop = rect.top - drag.transformY;
    drag.transformX = event.clientX - drag.grabOffsetX - slotLeft;
    drag.transformY = event.clientY - drag.grabOffsetY - slotTop;
    cell.style.transform = 'translate(' + drag.transformX + 'px, ' + drag.transformY + 'px)';
    positionSlot(cell);

    const index = insertionIndex(event.clientX, event.clientY, drag.tile);
    if (index === drag.index) return;

    drag.index = index;
    moveTileTo(drag.tile, index);

    // The reorder moved the slot under the tile: the tile has to follow the
    // pointer again.
    const moved = cell.getBoundingClientRect();
    const movedSlotLeft = moved.left - drag.transformX;
    const movedSlotTop = moved.top - drag.transformY;
    drag.transformX = event.clientX - drag.grabOffsetX - movedSlotLeft;
    drag.transformY = event.clientY - drag.grabOffsetY - movedSlotTop;
    cell.style.transform = 'translate(' + drag.transformX + 'px, ' + drag.transformY + 'px)';
    positionSlot(cell);
  }

  function onMovePointerUp() {
    if (!drag) return;
    const tile = drag.tile;
    tile.element.classList.remove('bento__cell--dragging');
    tile.element.classList.add('bento__cell--landed');
    tile.element.style.transform = '';
    hideSlot();
    document.removeEventListener('pointermove', onMovePointerMove);
    document.removeEventListener('pointerup', onMovePointerUp);
    document.removeEventListener('pointercancel', onMovePointerUp);
    drag = null;

    scheduleSave();
    announce(proviant.t("js.home.tileMoved", tile.label || tile.id, tile.order + 1));
  }

  function gridMetrics() {
    const styles = window.getComputedStyle(grid);
    const gap = parseFloat(styles.columnGap) || 0;
    const columns = currentColumns();
    const width = grid.getBoundingClientRect().width;
    const columnWidth = Math.max(1, (width - gap * (columns - 1)) / columns);
    const rowHeight = parseFloat(styles.getPropertyValue('--bento-row-height')) || 96;
    return { gap: gap, columnWidth: columnWidth, rowHeight: rowHeight };
  }

  function onResizePointerDown(event, handle) {
    const cell = handle.closest('[data-tile]');
    if (!cell) return;
    const tile = tileFor(cell);
    if (!tile) return;

    const metrics = gridMetrics();
    resize = {
      tile: tile,
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      startW: tile.w,
      startH: tile.h,
      columnWidth: metrics.columnWidth,
      gap: metrics.gap,
      rowHeight: metrics.rowHeight,
    };

    if (handle.setPointerCapture) handle.setPointerCapture(event.pointerId);
    cell.classList.remove('bento__cell--landed');
    cell.classList.add('bento__cell--resizing');
    document.addEventListener('pointermove', onResizePointerMove);
    document.addEventListener('pointerup', onResizePointerUp);
    document.addEventListener('pointercancel', onResizePointerUp);
    event.preventDefault();
  }

  function onResizePointerMove(event) {
    if (!resize) return;
    const metrics = resize;
    const deltaColumns = Math.round((event.clientX - metrics.startX) / (metrics.columnWidth + metrics.gap));
    const deltaRows = Math.round((event.clientY - metrics.startY) / (metrics.rowHeight + metrics.gap));
    const width = clamp(metrics.startW + deltaColumns, 1, MAX_COLUMNS);
    const height = clamp(metrics.startH + deltaRows, 1, MAX_ROWS);
    if (width === metrics.tile.w && height === metrics.tile.h) return;

    metrics.tile.w = width;
    metrics.tile.h = height;
    applyLayout();
  }

  function onResizePointerUp() {
    if (!resize) return;
    const tile = resize.tile;
    tile.element.classList.remove('bento__cell--resizing');
    tile.element.classList.add('bento__cell--landed');
    document.removeEventListener('pointermove', onResizePointerMove);
    document.removeEventListener('pointerup', onResizePointerUp);
    document.removeEventListener('pointercancel', onResizePointerUp);
    resize = null;

    scheduleSave();
    announce(proviant.t("js.home.tileResized", tile.label || tile.id, tile.w, tile.h));
  }

  function onHandleKeyDown(event) {
    const handle = event.target.closest('.bento__handle');
    if (!handle || !grid.contains(handle)) return;
    const cell = handle.closest('[data-tile]');
    if (!cell) return;
    const tile = tileFor(cell);
    if (!tile) return;

    const kind = handle.dataset.tileHandle;
    const position = orderedTiles().indexOf(tile);
    let handled = true;

    if (kind === 'move') {
      if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
        moveTileTo(tile, position + 1);
      } else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
        moveTileTo(tile, position - 1);
      } else {
        handled = false;
      }
      if (handled) {
        announce(proviant.t("js.home.tileMoved", tile.label || tile.id, tile.order + 1));
        scheduleSave();
      }
    } else {
      if (event.key === 'ArrowRight') {
        tile.w = clamp(tile.w + 1, 1, MAX_COLUMNS);
      } else if (event.key === 'ArrowLeft') {
        tile.w = clamp(tile.w - 1, 1, MAX_COLUMNS);
      } else if (event.key === 'ArrowDown') {
        tile.h = clamp(tile.h + 1, 1, MAX_ROWS);
      } else if (event.key === 'ArrowUp') {
        tile.h = clamp(tile.h - 1, 1, MAX_ROWS);
      } else {
        handled = false;
      }
      if (handled) {
        applyLayout();
        announce(proviant.t("js.home.tileResized", tile.label || tile.id, tile.w, tile.h));
        scheduleSave();
      }
    }

    if (handled) event.preventDefault();
  }

  function onResetClick() {
    storedLayout = null;
    clearLocalLayout();
    animateReflow(null, function () {
      readTiles();
      applyLayout();
    });
    setResetEnabled(false);
    saveLayout({ version: LAYOUT_VERSION, tiles: {} });
    announce(proviant.t("js.home.layoutReset"));
  }

  function applyStoredLayout(layout) {
    if (!layout || layout.version !== LAYOUT_VERSION || !layout.tiles) return false;
    storedLayout = layout;
    if (!grid) return true;
    readTiles();
    applyLayout();
    setResetEnabled(hasStoredArrangement());
    return true;
  }

  function initialize() {
    grid = document.getElementById('dashboard');
    if (!grid) return;

    readTiles();
    applyLayout();
    attachHandles();
    setResetEnabled(hasStoredArrangement());

    editing = false;
    applyEditMode();

    grid.addEventListener('pointerdown', function (event) {
      const handle = event.target.closest('.bento__handle');
      if (!handle || !grid.contains(handle)) return;
      if (handle.dataset.tileHandle === 'move') onMovePointerDown(event, handle);
      else if (handle.dataset.tileHandle === 'resize') onResizePointerDown(event, handle);
    });

    grid.addEventListener('keydown', onHandleKeyDown);

    const toggle = document.getElementById('dashboard-edit-layout');
    if (toggle) {
      toggle.disabled = false;
      toggle.addEventListener('click', function () { setEditing(!editing); });
    }

    const reset = document.getElementById('dashboard-reset-layout');
    if (reset) reset.addEventListener('click', onResetClick);
  }

  document.addEventListener('DOMContentLoaded', function () {
    // The stored arrangement arrives while the tiles are still being built:
    // the local copy paints the dashboard right away, the server copy follows.
    storedLayout = readLocalLayout();

    if (typeof proviant.getDashboardLayout !== 'function') return;
    proviant.getDashboardLayout().then(function (response) {
      if (response.code !== 200 || !response.message) return;
      applyStoredLayout(response.message);
    }).catch(function () {
      void 0;
    });
  });

  document.addEventListener('proviant:dashboardready', function () {
    initialize();
    if (storedLayout) {
      readTiles();
      applyLayout();
      setResetEnabled(hasStoredArrangement());
    }
  });

  document.addEventListener('keydown', function (event) {
    if (!editing || event.key !== 'Escape') return;
    setEditing(false);
    const toggle = document.getElementById('dashboard-edit-layout');
    if (toggle) toggle.focus();
  });

  window.addEventListener('resize', function () {
    if (!grid || tiles.length === 0) return;
    applyLayout();
    updateHandleVisibility();
  });
})();
