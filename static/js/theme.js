// theme.js - Instant Light/Dark Mode Switcher

(function () {
  const THEME_STORAGE_KEY = 'dorm_theme';

  // Get preferred theme from localStorage or OS system preference
  function getPreferredTheme() {
    const savedTheme = localStorage.getItem(THEME_STORAGE_KEY);
    if (savedTheme === 'light' || savedTheme === 'dark') {
      return savedTheme;
    }
    // Check OS system preference
    if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
      return 'dark';
    }
    return 'light';
  }

  // Apply theme to DOM
  function setTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem(THEME_STORAGE_KEY, theme);

    // Update toggle button icon and text if present
    const toggleBtn = document.getElementById('theme-toggle-btn');
    const toggleIcon = document.getElementById('theme-toggle-icon');
    const toggleText = document.getElementById('theme-toggle-text');

    if (toggleIcon) {
      if (theme === 'dark') {
        toggleIcon.className = 'bi bi-sun-fill text-warning';
        if (toggleText) toggleText.setAttribute('data-i18n', 'nav.themeLight');
      } else {
        toggleIcon.className = 'bi bi-moon-stars-fill text-primary';
        if (toggleText) toggleText.setAttribute('data-i18n', 'nav.themeDark');
      }
    }

    // Trigger re-translation of the toggle text if i18n is loaded
    if (window.i18n && typeof window.i18n.translatePage === 'function') {
      window.i18n.translatePage();
    }
  }

  // Toggle between light and dark
  window.toggleTheme = function () {
    const currentTheme = document.documentElement.getAttribute('data-theme') || 'light';
    const nextTheme = currentTheme === 'dark' ? 'light' : 'dark';
    setTheme(nextTheme);
  };

  // Immediate execution before DOM finishes loading to prevent flash of wrong theme
  const initialTheme = getPreferredTheme();
  document.documentElement.setAttribute('data-theme', initialTheme);

  // Once DOM is loaded, bind listener and update icons
  document.addEventListener('DOMContentLoaded', function () {
    setTheme(getPreferredTheme());

    const toggleBtn = document.getElementById('theme-toggle-btn');
    if (toggleBtn) {
      toggleBtn.addEventListener('click', function (e) {
        e.preventDefault();
        window.toggleTheme();
      });
    }

    // Listen to OS system theme changes
    if (window.matchMedia) {
      window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function (e) {
        if (!localStorage.getItem(THEME_STORAGE_KEY)) {
          setTheme(e.matches ? 'dark' : 'light');
        }
      });
    }
  });
})();
