(function() {
  'use strict';

  var STORAGE_KEY = 'proviant_theme';
  var THEMES = ['light', 'dark', 'system'];

  function getSystemTheme() {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }

  function getStoredTheme() {
    return localStorage.getItem(STORAGE_KEY) || 'system';
  }

  function getEffectiveTheme() {
    var stored = getStoredTheme();
    return stored === 'system' ? getSystemTheme() : stored;
  }

  function getThemeIcon(theme) {
    if (theme === 'dark') return 'bi-moon-fill';
    if (theme === 'light') return 'bi-sun-fill';
    return 'bi-circle-half';
  }

  function updateMetaThemeColor(theme) {
    var meta = document.querySelector('meta[name="theme-color"]');
    if (meta) {
      meta.setAttribute('content', theme === 'dark' ? '#2a2620' : '#3D7A5C');
    }
  }

  function updateToggleIcons(storedTheme) {
    var iconClass = 'bi ' + getThemeIcon(storedTheme);
    var desktopIcon = document.getElementById('themeToggleIcon');
    if (desktopIcon) desktopIcon.className = iconClass;
    var mobileIcon = document.getElementById('themeToggleIconMobile');
    if (mobileIcon) mobileIcon.className = 'bi me-2 ' + getThemeIcon(storedTheme);
  }

  function applyTheme(storedTheme) {
    var effective = storedTheme === 'system' ? getSystemTheme() : storedTheme;
    document.documentElement.setAttribute('data-bs-theme', effective);
    updateMetaThemeColor(effective);
    updateToggleIcons(storedTheme);
    document.dispatchEvent(new CustomEvent('proviant:themechange', { detail: { theme: storedTheme, effective: effective } }));
  }

  applyTheme(getStoredTheme());

  // Icons don't exist yet when this runs in <head> — refresh them once the DOM is parsed
  document.addEventListener('DOMContentLoaded', function() {
    updateToggleIcons(getStoredTheme());
  });

  var prefersDark = window.matchMedia('(prefers-color-scheme: dark)');
  function onSystemThemeChange() {
    if (getStoredTheme() === 'system') {
      applyTheme('system');
    }
  }
  if (prefersDark.addEventListener) {
    prefersDark.addEventListener('change', onSystemThemeChange);
  } else if (prefersDark.addListener) {
    prefersDark.addListener(onSystemThemeChange);
  }

  window.proviantTheme = {
    get: getStoredTheme,
    getEffective: getEffectiveTheme,
    set: function(theme) {
      if (THEMES.indexOf(theme) === -1) return;
      localStorage.setItem(STORAGE_KEY, theme);
      applyTheme(theme);
    },
    cycle: function() {
      var current = getStoredTheme();
      var next = THEMES[(THEMES.indexOf(current) + 1) % THEMES.length];
      this.set(next);
    },
    onChange: function(callback) {
      document.addEventListener('proviant:themechange', function(e) {
        callback(e.detail.effective, e.detail.theme);
      });
    }
  };

  document.addEventListener('click', function(event) {
    if (event.target.closest('#btnThemeToggle') || event.target.closest('#btnThemeToggleMobile')) {
      event.preventDefault();
      window.proviantTheme.cycle();
    }
  });
})();
