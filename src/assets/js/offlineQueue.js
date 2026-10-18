/* unified IndexedDB offline outbox */
(function () {
  'use strict';
  var proviant = window.proviant || (window.proviant = {});

  var DB_NAME = 'proviant-offline';
  var DB_VERSION = 1;
  var LEGACY_QUEUE_KEY = 'proviant_offline_queue';
  var LEGACY_ID_PREFIX = 'legacy-amount-';
  // A queued op the server keeps rejecting would otherwise retry on every page
  // load forever, with its error visible to nobody. Five attempts is well past
  // the transient rejections (502, a rejected batch) the queue rides out; a
  // request that never reached the server costs no attempt at all (transport()).
  var MAX_SYNC_ATTEMPTS = 5;
  // The outbox drains from a background timer, so a refusal has to be paced
  // through rather than treated as failure. This bounds how long one drain may
  // sit waiting on the server before it gives the rest of the queue back to the
  // next trigger.
  var MAX_THROTTLE_WAITS = 20;
  var MAX_RETRY_AFTER_SECONDS = 30;
  var MAX_THROTTLE_WINDOW_MS = 60000;
  // Mirrors util.ReceiptBulkMaxItems, the per-request item cap the sync endpoint
  // enforces. Exceeding it fails the whole batch, so the client has to split.
  var MAX_BATCH_ITEMS = 100;
  // The server injects its configured body cap with every page, so the
  // client splits at the value the endpoint actually enforces. The
  // constant is the fallback for a client cached before the cap became
  // configurable — the two can disagree only across an upgrade.
  var DEFAULT_MAX_BATCH_BYTES = 413696;
  var MAX_BATCH_BYTES =
    (window.proviantOffline && window.proviantOffline.maxBatchBytes) ||
    DEFAULT_MAX_BATCH_BYTES;
  // How long a drain waits before picking up a backlog it had to hand back.
  var REARM_DRAIN_SECONDS = 30;

  function openDB() {
    return new Promise(function (resolve, reject) {
      if (!('indexedDB' in window)) { reject(new Error('indexedDB unavailable')); return; }
      var req = indexedDB.open(DB_NAME, DB_VERSION);
      req.onupgradeneeded = function (event) {
        var db = event.target.result;
        if (!db.objectStoreNames.contains('outbox')) {
          var os = db.createObjectStore('outbox', { keyPath: 'id' });
          os.createIndex('createdAt', 'createdAt', { unique: false });
        }
        if (!db.objectStoreNames.contains('offCache')) {
          db.createObjectStore('offCache', { keyPath: 'barcode' });
        }
      };
      req.onsuccess = function (event) { resolve(event.target.result); };
      req.onerror = function (event) { reject(event.target.error); };
    });
  }

  // The promise is created eagerly so every caller shares one connection, but
  // openDB can reject synchronously when IndexedDB is missing entirely (a
  // private window in some browsers). Without a handler attached here the
  // rejection surfaces as an unhandled promise rejection at load, before the
  // first caller gets a chance to attach one.
  var dbPromise = openDB();
  dbPromise.catch(function () {});

  function runTx(storeNames, mode, fn) {
    return dbPromise.then(function (db) {
      return new Promise(function (resolve, reject) {
        var tx = db.transaction(storeNames, mode);
        var stores = {};
        storeNames.forEach(function (name) { stores[name] = tx.objectStore(name); });
        var result;
        try { result = fn(stores, tx); } catch (err) { tx.abort(); reject(err); return; }
        tx.oncomplete = function () {
          if (result && typeof result.then === 'function') { result.then(resolve, reject); }
          else resolve(result);
        };
        tx.onerror = function () { reject(tx.error); };
        tx.onabort = function () { reject(tx.error || new Error('transaction aborted')); };
      });
    });
  }

  function reqToPromise(req) {
    return new Promise(function (resolve, reject) {
      req.onsuccess = function (event) { resolve(event.target.result); };
      req.onerror = function (event) { reject(event.target.error); };
    });
  }

  function getAll(store) {
    return runTx([store], 'readonly', function (stores) {
      return reqToPromise(stores[store].getAll());
    });
  }

  function get(store, key) {
    return runTx([store], 'readonly', function (stores) {
      return reqToPromise(stores[store].get(key));
    });
  }

  function put(store, value) {
    return runTx([store], 'readwrite', function (stores) {
      return reqToPromise(stores[store].put(value));
    });
  }

  function remove(store, key) {
    return runTx([store], 'readwrite', function (stores) {
      return reqToPromise(stores[store].delete(key));
    });
  }

  function generateId() {
    try { return crypto.randomUUID(); } catch (_) { void 0; }
    return 'offline-' + Date.now() + '-' + Math.random().toString(36).slice(2);
  }

  // The outbox id is derived from the legacy entry rather than generated, so a
  // migration that dies half way is safe to repeat: re-running it overwrites the
  // same ids instead of adding a second copy of every op it had already
  // imported, which the server would then apply twice as fresh mutations.
  //
  // Deriving from the product id only holds while it is unique in the array. The
  // legacy queue merged repeated deltas per product, so a duplicate should not
  // exist — but if one does, keying both entries on it makes the second
  // overwrite the first and silently drop a delta the user made. Such a pair
  // falls back to the position, which is unique per entry and just as stable
  // across re-runs.
  function legacyIds(entries) {
    var perProduct = Object.create(null);
    for (var i = 0; i < entries.length; i++) {
      var entry = entries[i] || {};
      if (entry.productID !== undefined && entry.productID !== null) {
        perProduct[entry.productID] = (perProduct[entry.productID] || 0) + 1;
      }
    }
    return entries.map(function (entry, index) {
      var productID = (entry || {}).productID;
      if (productID !== undefined && productID !== null && perProduct[productID] === 1) {
        return LEGACY_ID_PREFIX + productID;
      }
      return LEGACY_ID_PREFIX + 'index-' + index;
    });
  }

  async function migrateLegacyQueue() {
    var raw;
    try { raw = localStorage.getItem(LEGACY_QUEUE_KEY); } catch (_) { return; }
    if (!raw) return;
    var parsed;
    try { parsed = JSON.parse(raw); } catch (_) { return; }
    if (!Array.isArray(parsed)) return;

    var ids = legacyIds(parsed);
    for (var i = 0; i < parsed.length; i++) {
      var op = parsed[i] || {};
      await enqueue({
        id: ids[i],
        type: 'updateAmount',
        payload: { productID: op.productID, delta: op.delta },
        createdAt: op.ts || Date.now(),
        lastError: null,
      });
    }
    try {
      localStorage.setItem(LEGACY_QUEUE_KEY + '_backup', raw);
      localStorage.removeItem(LEGACY_QUEUE_KEY);
    } catch (_) { void 0; }
  }

  async function enqueue(op) {
    var item = {
      id: op.id || generateId(),
      type: op.type,
      payload: op.payload || {},
      status: 'pending',
      createdAt: op.createdAt || Date.now(),
      updatedAt: Date.now(),
      retryCount: 0,
      lastError: op.lastError || null,
    };
    if (item.type === 'createProduct' && !item.payload.clientSyncId) {
      item.payload.clientSyncId = item.id;
    }
    if (item.type === 'updateAmount' && !item.payload.clientMutationId) {
      item.payload.clientMutationId = item.id;
    }
    await put('outbox', item);
    await updateBadge();
    // An op can be enqueued while online (the request threw after the page was
    // up): no online event is coming for it, so without a scheduled drain it
    // sits until the next navigation while the badge shows it as pending.
    if (navigator.onLine) scheduleDrain(REARM_DRAIN_SECONDS);
    return item.id;
  }

  // IndexedDB can be missing entirely (a private window in some browsers), and
  // every caller of these wrappers chains .then with no catch — a rejected
  // enqueue would be an unhandled promise with no feedback at all. Report a
  // failure the pages already render instead.
  async function enqueueOrReport(op, queuedCode, extra) {
    try {
      await enqueue(op);
      return Object.assign({ code: queuedCode, message: 'queued', deleted: false, queued: true }, extra || {});
    } catch (_) {
      return {
        code: 503,
        message: proviant.t ? proviant.t('js.app.offlineStorageUnavailable') : 'offline storage unavailable',
        deleted: false,
        queued: false,
      };
    }
  }

  function byCreatedAt(a, b) {
    return a.createdAt - b.createdAt;
  }

  async function pendingItems() {
    var items = await getAll('outbox');
    // 'syncing' is included on purpose: a drain interrupted by a tab close
    // leaves the op in that state, and nothing else will ever move it again.
    // Not re-sending it is sync()'s in-flight guard, not this filter.
    return items
      .filter(function (item) { return item.status !== 'failed'; })
      .sort(byCreatedAt);
  }

  async function failedItems() {
    var items = await getAll('outbox');
    return items.filter(function (item) { return item.status === 'failed'; }).sort(byCreatedAt);
  }

  async function discard(id) {
    await remove('outbox', id);
    await updateBadge();
  }

  async function retryFailed(id) {
    var item = await get('outbox', id);
    if (!item) return;
    item.status = 'pending';
    item.retryCount = 0;
    item.lastError = null;
    item.updatedAt = Date.now();
    await put('outbox', item);
    await updateBadge();
    // Same reason as enqueue: a retry the user starts while online has no
    // online event behind it.
    if (navigator.onLine) scheduleDrain(REARM_DRAIN_SECONDS);
  }

  // A refusal is not a failure: it costs no attempt and leaves the ops pending,
  // so a burst-limited endpoint cannot walk a whole outbox into the failed state.
  function throttled(retryAfterSeconds) {
    var err = new Error('rate limited, retrying in ' + retryAfterSeconds + 's');
    err.throttled = true;
    err.retryAfterSeconds = retryAfterSeconds;
    return err;
  }

  // A request that never reached the server says nothing about whether the op
  // is acceptable, so it costs no attempt: the drain stops and the re-arm picks
  // the backlog up later, instead of walking healthy ops into 'failed' over a
  // flaky connection. The error is rebuilt rather than tagged because the
  // thrower may live in another realm, where instanceof Error is false.
  function transport(err) {
    var wrapped = new Error(err && err.message ? err.message : String(err));
    wrapped.transport = true;
    return wrapped;
  }

  function parseRetryAfter(value) {
    var seconds = Number.parseInt(value, 10);
    if (!isFinite(seconds) || seconds <= 0) seconds = 1;
    return Math.min(seconds, MAX_RETRY_AFTER_SECONDS);
  }

  function retryAfterSeconds(res) {
    var header = res.headers && res.headers.get ? res.headers.get('Retry-After') : null;
    return parseRetryAfter(header);
  }

  function delayFromError(err) {
    return err && err.throttled ? err.retryAfterSeconds || 1 : null;
  }

  // A 401/403 is the session or the CSRF cookie being rejected, not a verdict on
  // the draft: signing in fixes it. A background drain that runs after the token
  // expired would otherwise park a whole backlog the user only has to log in to
  // send, and every retry would burn an attempt toward the same verdict.
  function isAuthRejection(err) {
    return err && (err.status === 401 || err.status === 403);
  }

  // A refusal is a verdict on the request, not a transient blip: the same body
  // earns the same 4xx, so retrying it only walks the backlog into 'failed'.
  // Auth rejections are isAuthRejection's case: the drain hands the backlog
  // back rather than spending it.
  function isRefusal(err) {
    if (isAuthRejection(err)) return false;
    return typeof err.status === 'number' && err.status >= 400 && err.status < 500;
  }

  // A 5xx is the server's problem, not the delta's: the next op would
  // earn the same verdict, so spending its attempt too only walks the
  // backlog into the failed state one op at a time.
  function isServerError(err) {
    return typeof err.status === 'number' && err.status >= 500;
  }

  // Honours Retry-After within a fixed budget of waits *and* wall-clock. The
  // count alone is not a bound: a server asking for the per-wait maximum can
  // keep a drain asleep long enough that the sync promise — which every manual
  // retry joins behind — outlives the user's patience.
  function ThrottlePacing() {
    this.waits = 0;
    this.deadline = Date.now() + MAX_THROTTLE_WINDOW_MS;
  }

  // Resolves true when the caller should wait and try again, false when the
  // drain has spent its budget and should hand the backlog to the re-arm.
  ThrottlePacing.prototype.awaitRetryAfter = async function (seconds) {
    if (this.waits >= MAX_THROTTLE_WAITS || Date.now() >= this.deadline) return false;
    this.waits++;
    await sleep(Math.min(seconds, Math.max(0, this.deadline - Date.now()) / 1000));
    return true;
  };

  function sleep(seconds) {
    return new Promise(function (resolve) { setTimeout(resolve, seconds * 1000); });
  }

  async function recordFailure(item, err) {
    item.retryCount = (item.retryCount || 0) + 1;
    item.status = item.retryCount >= MAX_SYNC_ATTEMPTS ? 'failed' : 'pending';
    item.lastError = err && err.message ? err.message : String(err);
    item.updatedAt = Date.now();
    await put('outbox', item);
  }

  // An op no attempt can fix skips the counter and goes straight to failed, so
  // it waits for the user instead of spending five drains to get there.
  async function park(item, err) {
    item.status = 'failed';
    item.lastError = err && err.message ? err.message : String(err);
    item.updatedAt = Date.now();
    await put('outbox', item);
  }

  // Returns the outcome the server reported for each op, or throws. A batch is
  // retried whole on a transport failure, and the server's own clientSyncId
  // idempotency absorbs the ops of that retry which did land.
  async function syncCreateBatch(items) {
    var drafts = items.map(function (item) {
      return Object.assign({}, item.payload, { clientSyncId: item.payload.clientSyncId || item.id });
    });
    var res;
    try {
      res = await fetch('/api/v1/products/sync-offline', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ items: drafts }),
      });
    } catch (err) {
      throw transport(err);
    }
    if (res.status === 429) throw throttled(retryAfterSeconds(res));

    var body;
    try { body = await res.json(); } catch (_) { body = null; }
    if (!res.ok) {
      const refused = new Error((body && body.message) || ('HTTP ' + res.status));
      refused.status = res.status;
      throw refused;
    }

    // Matched by position: the endpoint reports one result per submitted item,
    // in order, each carrying its index. Keyed on clientSyncId instead, a result
    // that arrived without one would look up nothing and the op would be
    // silently dropped as if it had synced.
    return items.map(function (item, position) {
      return { item: item, reported: (body.results && body.results[position]) || null };
    });
  }

  async function settleCreates(settled) {
    for (var i = 0; i < settled.length; i++) {
      var item = settled[i].item;
      var reported = settled[i].reported;
      if (reported && (reported.status === 'created' || reported.status === 'already-synced')) {
        await remove('outbox', item.id);
        continue;
      }
      // A result that never arrived is a failure, not a success: dropping the
      // op as synced would lose the product the server may never have seen.
      const failure = new Error((reported && reported.reason) || 'server did not report this item');
      // The server marks a reason it cannot fix by resubmitting the same draft
      // (a blank sync id, one held by another member's private product). Parking
      // spends no retry budget on it; the retry button still offers it back.
      if (reported && reported.terminal) {
        await park(item, failure);
        continue;
      }
      await recordFailure(item, failure);
    }
  }

  async function sendAmountDelta(item) {
    // Only proviant.js tags transport failures; a rejected body after an
    // answered response stays a plain error and costs an attempt like any
    // other server answer.
    var result = await proviant._sendAmountDelta(item.payload.productID, item.payload.delta, item.payload.clientMutationId);
    // _sendAmountDelta resolves for every JSON error body, so its status is the
    // only signal that the delta was not applied. Dropping the op on a non-2xx
    // would lose the mutation outright.
    if (result.code === 429) throw throttled(parseRetryAfter(result.retryAfter));
    if (result.code === 404) {
      // 404 means the product is gone *or* belongs to another household — the
      // handler answers both alike on purpose, so the two cannot be told apart
      // here. Neither can ever succeed, so the op is dropped in both cases.
      await remove('outbox', item.id);
      return;
    }
    if (result.code < 200 || result.code >= 300) {
      const refused = new Error('HTTP ' + result.code);
      refused.status = result.code;
      throw refused;
    }
    await remove('outbox', item.id);
  }

  // Returns { wait } to pause before the next op, { stop } to hand the rest of
  // the drain back to the re-arm, and {} when there is no reason to pause.
  async function drainAmountDelta(item) {
    item.status = 'syncing';
    item.updatedAt = Date.now();
    await put('outbox', item);
    try {
      await sendAmountDelta(item);
      return {};
    } catch (err) {
      if (isAuthRejection(err)) {
        item.status = 'pending';
        item.lastError = err.message;
        item.updatedAt = Date.now();
        await put('outbox', item);
        return { stop: true };
      }
      if (err.transport || err.throttled) {
        item.status = 'pending';
        item.lastError = err.message;
        item.updatedAt = Date.now();
        await put('outbox', item);
        if (err.transport) return { stop: true };
        return { wait: err.retryAfterSeconds || 1 };
      }
      if (isServerError(err)) {
        // One attempt for the op that saw the outage, then the rest of
        // the backlog waits for the re-arm instead of spending an
        // attempt each on the same dead server.
        await recordFailure(item, err);
        return { stop: true };
      }
      if (isRefusal(err)) {
        await park(item, err);
        return {};
      }
      await recordFailure(item, err);
      return {};
    }
  }

  // Takes the next batch, stopping at whichever of the two server-side caps
  // binds first. Measured from the payloads actually sent rather than assumed
  // from the item count, so one fat draft cannot refuse the ones behind it. A
  // single draft over the byte cap still goes out alone, where the server's own
  // refusal reason is what the user gets to see.
  function takeBatch(creates, payloadBytes, from) {
    var batch = [];
    var bytes = 2;
    for (var i = from; i < creates.length && batch.length < MAX_BATCH_ITEMS; i++) {
      if (batch.length > 0 && bytes + payloadBytes[i] > MAX_BATCH_BYTES) break;
      bytes += payloadBytes[i];
      batch.push(creates[i]);
    }
    return batch;
  }

  // The server cap is a byte cap, so the split has to be measured in bytes too:
  // a name in any non-Latin script is several bytes per character, and a split
  // measured in characters undercounts it straight into a refusal.
  function utf8ByteLength(value) {
    if (typeof TextEncoder !== 'function') return value.length;
    if (!utf8ByteLength.encoder) utf8ByteLength.encoder = new TextEncoder();
    return utf8ByteLength.encoder.encode(value).length;
  }

  // Returns true when it handed the rest of the drain back — throttled out of
  // budget or the connection gone — so the caller does not start the deltas
  // against the same refusing server.
  async function drainCreates(creates) {
    var pacing = new ThrottlePacing();
    // The serialized draft plus room for the envelope fields the payload does not
    // carry; over-estimating is free, under-estimating gets the batch refused.
    var payloadBytes = creates.map(function (item) { return utf8ByteLength(JSON.stringify(item.payload)) + 64; });
    var batchStart = 0;
    while (batchStart < creates.length) {
      var batch = takeBatch(creates, payloadBytes, batchStart);
      var fetched;
      try {
        fetched = await syncCreateBatch(batch);
      } catch (err) {
        // The connection went away mid-drain: stop and let the re-arm retry,
        // rather than spending an attempt per batch on a network that is not
        // there.
        if (err.transport) return true;
        // An auth rejection hands the whole backlog back for the same reason:
        // the session was refused, not these drafts.
        if (isAuthRejection(err)) return true;
        var wait = delayFromError(err);
        if (wait !== null) {
          if (!await pacing.awaitRetryAfter(wait)) return true;
          continue;
        }
        // A 4xx is a verdict on the request itself — an over-sized body, an
        // unparseable one, an expired session — so resubmitting this exact
        // batch cannot change it. Retrying it would park the whole backlog on
        // the fifth attempt and tell the user nothing until then; a park says
        // why straight away, and the retry button still offers it back.
        var refused = isRefusal(err);
        for (var failed = 0; failed < batch.length; failed++) {
          await (refused ? park(batch[failed], err) : recordFailure(batch[failed], err));
        }
        // A 5xx is the server's problem, not the request's, so this batch keeps
        // a retry to spend. But it costs only *this* batch: falling through to
        // the next batch would spend a second attempt of every remaining batch
        // on the same outage, and a single proxy blip then walks the whole
        // backlog into 'failed'. Hand the rest back to the re-arm instead, so a
        // blip costs one attempt and a real outage still parks what it reached.
        if (!refused) return true;
        batchStart += batch.length;
        continue;
      }
      // Outside the try: an IndexedDB failure while settling is a storage
      // fault, not the server refusing, and must not be charged to the batch
      // as a failed attempt on every item — including the ones already removed.
      await settleCreates(fetched);
      batchStart += batch.length;
    }
    return false;
  }

  async function drainDeltas(deltas) {
    var pacing = new ThrottlePacing();
    for (var i = 0; i < deltas.length; i++) {
      var outcome = await drainAmountDelta(deltas[i]);
      if (outcome.stop) return;
      if (!outcome.wait) continue;
      if (!await pacing.awaitRetryAfter(outcome.wait)) return;
    }
  }

  async function drain() {
    if (!navigator.onLine) return;
    try {
      var items = await pendingItems();
      var creates = [];
      var deltas = [];
      var unknown = [];
      items.forEach(function (item) {
        if (item.type === 'createProduct') creates.push(item);
        else if (item.type === 'updateAmount') deltas.push(item);
        else unknown.push(item);
      });

      // One request per queued create is what this used to do, and against a low
      // request budget it meant every create after the first was refused. The
      // endpoint already takes a whole batch, so a backlog of creates costs one
      // call. Amount deltas have no batch endpoint and stay one request each.
      var handedBack = await drainCreates(creates);
      if (!handedBack) await drainDeltas(deltas);
      // An op this build cannot execute is a change the user still owns, so it is
      // parked for them to see and discard rather than deleted — a renamed type in
      // a newer build would otherwise lose queued work silently.
      for (var u = 0; u < unknown.length; u++) {
        await park(unknown[u], new Error('unsupported op type "' + unknown[u].type + '"'));
      }
    } finally {
      // The body can throw — an IndexedDB fault while settling a batch, say —
      // and a half-drained outbox still owes itself a retry: without this, the
      // remainder waits for the next page load, online event or enqueue
      // instead of the re-arm timer.
      await updateBadge();
      var remaining;
      try { remaining = await pendingItems(); } catch (_) { remaining = null; }
      if (remaining !== null && remaining.length > 0 && navigator.onLine) {
        scheduleDrain(REARM_DRAIN_SECONDS);
      }
    }
  }

  // One drain runs at a time. The online event and the startup timer can land
  // together, and without this both would walk the same outbox, sending every op
  // twice against a limiter that admits very few requests.
  var draining = null;
  var rearmTimer = null;

  // A drain can hand the rest of the backlog back — throttled out of budget, or
  // the connection gone — but the only other triggers are page load and the
  // online event, so without re-arming here the remainder sits unsynced until
  // the user happens to navigate again.
  function scheduleDrain(seconds) {
    if (rearmTimer) clearTimeout(rearmTimer);
    rearmTimer = setTimeout(function () {
      rearmTimer = null;
      sync().catch(function () {});
    }, seconds * 1000);
  }

  async function sync() {
    if (draining) return draining;
    if (rearmTimer) { clearTimeout(rearmTimer); rearmTimer = null; }
    // engine.ready: the legacy import has to land before the first drain, or a
    // flush right after load drains the outbox it has not finished filling.
    draining = Promise.resolve(engine.ready).then(drain).finally(function () { draining = null; });
    return draining;
  }

  async function updateBadge() {
    try {
      var items = await getAll('outbox');
      var pending = 0;
      var failed = 0;
      var lastError = null;
      items.forEach(function (item) {
        if (item.status === 'failed') { failed++; if (!lastError) lastError = item.lastError; }
        else pending++;
      });
      proviant.offline.pendingCount = pending;
      proviant.offline.failedCount = failed;
      // The banner is the only place a stuck change is visible, so it has to be
      // re-evaluated here — while main.js may not have loaded yet, hence the guard.
      if (typeof globalThis.updateOfflineBanner === 'function') globalThis.updateOfflineBanner();
      var retryBtn = document.getElementById('offlineQueueRetry');
      var discardBtn = document.getElementById('offlineQueueDiscard');
      if (retryBtn) retryBtn.classList.toggle('d-none', failed === 0);
      if (discardBtn) discardBtn.classList.toggle('d-none', failed === 0);
      var el = document.getElementById('offlineQueueCount');
      if (!el) return;
      if (pending + failed > 0) {
        var text = proviant.tn
          ? proviant.tn('js.app.offlineQueuePending', pending)
          : (pending + ' pending');
        el.textContent = failed > 0 && proviant.t
          ? proviant.t('js.app.offlineQueueFailed', text, failed)
          : text;
        // lastError is the only place the reason a permanently failing op was
        // parked ever reaches the user, so it rides along as the tooltip.
        el.title = lastError || '';
        el.classList.remove('d-none');
      } else {
        el.classList.add('d-none');
      }
    } catch (_) { void 0; }
  }

  async function cacheOpenFoodFacts(barcode, data) {
    if (!barcode) return;
    // Best-effort: this write follows an answered lookup, so a storage fault
    // must not turn the response already in hand into a failed scan.
    try {
      await put('offCache', { barcode: barcode, data: data, cachedAt: Date.now() });
    } catch (_) { void 0; }
  }
  async function getCachedOpenFoodFacts(barcode) {
    try {
      var hit = await get('offCache', barcode);
      return hit && hit.data ? hit.data : null;
    } catch (_) { return null; }
  }

  var engine = {
    enqueue: enqueue,
    sync: sync,
    pendingItems: pendingItems,
    failedItems: failedItems,
    discard: discard,
    retryFailed: retryFailed,
    cacheOpenFoodFacts: cacheOpenFoodFacts,
    getOpenFoodFacts: getCachedOpenFoodFacts,
    generateId: generateId,
    pendingCount: 0,
    failedCount: 0,
  };

  proviant.offline = engine;

  // Resolves once the legacy import has settled. Without it a caller that
  // flushes the outbox right after load races the migration and drains half of
  // it, leaving the rest for a later pass.
  engine.ready = migrateLegacyQueue().then(function () { return updateBadge(); }).catch(function () {});

  if (typeof proviant.createProduct === 'function') {
    var originalCreateProduct = proviant.createProduct;
    proviant.createProduct = async function (barcode, expireAt, amount, storageLocationId, isPrivate, clientSyncIdArg, unit) {
      // The sync id goes out on the online path too. Without it the server-side
      // idempotency check cannot fire, so a response lost after the server
      // committed — which is exactly what the catch below detects — would enqueue
      // a second create for a product that already exists.
      var clientSyncId = generateId();
      var draft = { barcode: barcode, expireAt: expireAt, amount: amount || 1, unit: unit || '', storageLocationId: storageLocationId || null, isPrivate: !!isPrivate, clientSyncId: clientSyncId };
      if (!navigator.onLine) {
        return enqueueOrReport({ id: clientSyncId, type: 'createProduct', payload: draft }, 202, { clientSyncId: clientSyncId });
      }
      try {
        return await originalCreateProduct(barcode, expireAt, amount, storageLocationId, isPrivate, clientSyncId, unit);
      } catch (_) {
        return enqueueOrReport({ id: clientSyncId, type: 'createProduct', payload: draft }, 202, { clientSyncId: clientSyncId });
      }
    };
  }

  if (typeof proviant.updateProductAmount === 'function') {
    var originalUpdateProductAmount = proviant.updateProductAmount;
    proviant.updateProductAmount = async function (productID, delta) {
      // The mutation id is generated up front so the online request and any
      // queued replay of it carry the same token: a response lost after the
      // server applied the delta is then a no-op on retry, not a second
      // increment.
      var clientMutationId = generateId();
      if (!navigator.onLine) {
        return enqueueOrReport({ type: 'updateAmount', payload: { productID: productID, delta: delta, clientMutationId: clientMutationId } }, 200);
      }
      try {
        return await originalUpdateProductAmount(productID, delta, clientMutationId);
      } catch (_) {
        return enqueueOrReport({ type: 'updateAmount', payload: { productID: productID, delta: delta, clientMutationId: clientMutationId } }, 200);
      }
    };
  }

  // The barcode lookup is the one product read the cached list snapshot cannot
  // answer, so an earlier response is the fallback rather than a write-only
  // store: a scan that was looked up while online still resolves offline.
  if (typeof proviant.getOpenFoodFactsData === 'function') {
    var originalOFF = proviant.getOpenFoodFactsData;
    proviant.getOpenFoodFactsData = async function (barcode) {
      try {
        var result = await originalOFF(barcode);
        if (result && result.code === 200 && result.message) {
          await cacheOpenFoodFacts(barcode, result.message);
          return result;
        }
        var cachedAnswer = await getCachedOpenFoodFacts(barcode);
        return cachedAnswer ? { code: 200, message: cachedAnswer, offline: true } : result;
      } catch (err) {
        var cachedOnError = await getCachedOpenFoodFacts(barcode);
        if (cachedOnError) return { code: 200, message: cachedOnError, offline: true };
        throw err;
      }
    };
  }

  proviant._flushQueue = sync;
  proviant._updateOfflineBadge = updateBadge;

  // The banner is shared markup on every page, so the controls are delegated
  // rather than bound at load: the buttons exist before this script runs.
  document.addEventListener('click', function (event) {
    if (event.target.closest('#offlineQueueRetry')) {
      retryAllFailed();
      return;
    }
    if (event.target.closest('#offlineQueueDiscard')) {
      discardFailedAfterConfirm();
    }
  });

  async function retryAllFailed() {
    var items = await failedItems();
    for (var i = 0; i < items.length; i++) {
      await retryFailed(items[i].id);
    }
    // retryFailed only re-arms the drain, and a click means the user wants the
    // attempt now rather than after REARM_DRAIN_SECONDS.
    if (navigator.onLine) sync().catch(function () {});
  }

  async function discardFailedAfterConfirm() {
    var items = await failedItems();
    if (!items.length || typeof proviant.showConfirm !== 'function') return;
    proviant.showConfirm(
      proviant.t('js.app.offlineDiscardTitle'),
      proviant.tn('js.app.offlineDiscardConfirm', items.length),
      function () {
        Promise.all(items.map(function (item) { return discard(item.id); })).catch(function () {});
      },
      proviant.t('common.discard'),
      'danger'
    );
  }

  function cacheProductSnapshot(products) {
    try { localStorage.setItem('proviant_product_snapshot', JSON.stringify(products || [])); } catch (_) { void 0; }
  }
  function readProductSnapshot() {
    try { return JSON.parse(localStorage.getItem('proviant_product_snapshot') || '[]'); } catch (_) { return []; }
  }
  function cachedProducts() {
    return { code: 200, products: readProductSnapshot(), message: 'cached', offline: true };
  }

  if (typeof proviant.getProducts === 'function') {
    var originalGetProducts = proviant.getProducts;
    proviant.getProducts = async function () {
      if (!navigator.onLine) {
        return cachedProducts();
      }
      try {
        var res = await originalGetProducts();
        if (res && res.code === 200 && Array.isArray(res.products)) cacheProductSnapshot(res.products);
        return res;
      } catch (_) {
        return cachedProducts();
      }
    };
  }

  window.addEventListener('online', function () { sync().catch(function () {}); });
  if (navigator.onLine) { setTimeout(function () { sync().catch(function () {}); }, 1500); }
})();