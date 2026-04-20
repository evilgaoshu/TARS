import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

import enUS from '../locales/en-US.json';
import zhCN from '../locales/zh-CN.json';

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      'en': { translation: enUS },
      'en-US': { translation: enUS },
      'zh': { translation: zhCN },
      'zh-CN': { translation: zhCN },
    },
    fallbackLng: 'zh',
    supportedLngs: ['zh', 'zh-CN', 'en', 'en-US'],
    keySeparator: false,
    interpolation: {
      escapeValue: false,
    },
    detection: {
      order: ['querystring', 'localStorage', 'cookie', 'navigator', 'htmlTag'],
      lookupLocalStorage: 'tars_lang',
      caches: ['localStorage', 'cookie'],
    },
  });

export default i18n;
