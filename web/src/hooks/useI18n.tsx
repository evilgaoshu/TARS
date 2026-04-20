import React, { useMemo } from 'react';
import { I18nextProvider, useTranslation } from 'react-i18next';
import i18n from '../lib/i18n';

type Language = 'en-US' | 'zh-CN';

export const I18nProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  return <I18nextProvider i18n={i18n}>{children}</I18nextProvider>;
};

// eslint-disable-next-line react-refresh/only-export-components
export const useI18n = () => {
  const { i18n: instance, t: translate } = useTranslation();

  return useMemo(() => ({
    lang: (instance.resolvedLanguage || instance.language || 'zh-CN') as Language,
    setLang: async (lang: Language) => {
      await instance.changeLanguage(lang);
      document.documentElement.lang = lang;
      localStorage.setItem('tars_lang', lang);
    },
    t: (key: string, paramsOrFallback?: Record<string, unknown> | string) => {
      if (typeof paramsOrFallback === 'string') {
        return translate(key, { defaultValue: paramsOrFallback });
      }
      return translate(key, paramsOrFallback as Record<string, unknown>);
    },
  }), [instance, translate]);
};
