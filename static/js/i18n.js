// i18n.js - Bilingual Thai / English Internationalization Engine

(function () {
  const LANG_STORAGE_KEY = 'dorm_lang';
  const DEFAULT_LANG = 'th';

  const translations = {
    th: null,
    en: null,
  };

  // Helper to resolve nested keys like "nav.home"
  function resolveKey(obj, path) {
    if (!obj || !path) return null;
    return path.split('.').reduce((acc, part) => (acc && acc[part] !== undefined ? acc[part] : null), obj);
  }

  // Load locale JSON file
  async function loadLocale(lang) {
    if (translations[lang]) {
      return translations[lang];
    }
    try {
      const response = await fetch(`/locales/${lang}.json`);
      if (!response.ok) throw new Error(`HTTP error ${response.status}`);
      const data = await response.json();
      translations[lang] = data;
      return data;
    } catch (err) {
      console.warn(`[i18n] Failed to load /locales/${lang}.json:`, err);
      return null;
    }
  }

  // Get current active language
  function getCurrentLang() {
    const saved = localStorage.getItem(LANG_STORAGE_KEY);
    return saved === 'en' || saved === 'th' ? saved : DEFAULT_LANG;
  }

  // Translate entire page elements with data-i18n and data-i18n-placeholder
  function translatePage() {
    const lang = getCurrentLang();
    const dict = translations[lang];
    if (!dict) return;

    // 1. Text elements
    document.querySelectorAll('[data-i18n]').forEach((el) => {
      const key = el.getAttribute('data-i18n');
      const val = resolveKey(dict, key);
      if (val !== null && val !== undefined) {
        // If element contains child icons or inputs, preserve them
        const icon = el.querySelector('i');
        if (icon) {
          el.innerHTML = '';
          el.appendChild(icon);
          el.appendChild(document.createTextNode(' ' + val));
        } else {
          el.textContent = val;
        }
      }
    });

    // 2. Placeholders
    document.querySelectorAll('[data-i18n-placeholder]').forEach((el) => {
      const key = el.getAttribute('data-i18n-placeholder');
      const val = resolveKey(dict, key);
      if (val !== null && val !== undefined) {
        el.setAttribute('placeholder', val);
      }
    });

    // 3. Titles / tooltips
    document.querySelectorAll('[data-i18n-title]').forEach((el) => {
      const key = el.getAttribute('data-i18n-title');
      const val = resolveKey(dict, key);
      if (val !== null && val !== undefined) {
        el.setAttribute('title', val);
      }
    });

    // Update active state in language selector UI
    document.querySelectorAll('.lang-btn').forEach((btn) => {
      const btnLang = btn.getAttribute('data-lang');
      if (btnLang === lang) {
        btn.classList.add('fw-bold', 'text-primary');
      } else {
        btn.classList.remove('fw-bold', 'text-primary');
      }
    });

    const activeLangLabel = document.getElementById('current-lang-label');
    if (activeLangLabel) {
      activeLangLabel.textContent = lang === 'th' ? '🇹🇭 TH' : '🇬🇧 EN';
    }
  }

  // Public translation function
  window.t = function (key, fallback = '') {
    const lang = getCurrentLang();
    const dict = translations[lang];
    const val = resolveKey(dict, key);
    return val !== null && val !== undefined ? val : fallback || key;
  };

  // Switch language
  window.setLanguage = async function (lang) {
    if (lang !== 'th' && lang !== 'en') return;
    localStorage.setItem(LANG_STORAGE_KEY, lang);
    await loadLocale(lang);
    translatePage();
  };

  window.i18n = {
    setLanguage: window.setLanguage,
    getCurrentLang: getCurrentLang,
    translatePage: translatePage,
    t: window.t,
  };

  // Initial load
  document.addEventListener('DOMContentLoaded', async function () {
    const lang = getCurrentLang();
    await loadLocale('th');
    await loadLocale('en');
    translatePage();

    // Attach click listeners to language switchers
    document.querySelectorAll('[data-lang-switch]').forEach((btn) => {
      btn.addEventListener('click', function (e) {
        e.preventDefault();
        const targetLang = this.getAttribute('data-lang-switch');
        window.setLanguage(targetLang);
      });
    });
  });
})();
