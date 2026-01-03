import React, { createContext, useContext, useState, useCallback } from 'react';
import { translations, Language, TranslationKey, AVAILABLE_LANGUAGES } from './generated';

interface TranslationContextType {
  t: (key: TranslationKey, defaultValue?: string) => string;
  language: Language;
  setLanguage: (lang: Language) => void;
  availableLanguages: Language[];
}

const TranslationContext = createContext<TranslationContextType | null>(null);

const STORAGE_KEY = 'csd-devtrack-language';

interface TranslationProviderProps {
  children: React.ReactNode;
}

export const TranslationProvider: React.FC<TranslationProviderProps> = ({ children }) => {
  const [language, setLanguageState] = useState<Language>(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY);
      if (stored && AVAILABLE_LANGUAGES.includes(stored as Language)) {
        return stored as Language;
      }
    } catch {
      // localStorage not available
    }
    return 'en';
  });

  const setLanguage = useCallback((lang: Language) => {
    setLanguageState(lang);
    try {
      localStorage.setItem(STORAGE_KEY, lang);
    } catch {
      // localStorage not available
    }
  }, []);

  const t = useCallback((key: TranslationKey, defaultValue?: string): string => {
    const langTranslations = translations[language];
    if (langTranslations && key in langTranslations) {
      return langTranslations[key as keyof typeof langTranslations];
    }
    // Fallback to English
    const enTranslations = translations.en;
    if (key in enTranslations) {
      return enTranslations[key as keyof typeof enTranslations];
    }
    return defaultValue || key;
  }, [language]);

  return (
    <TranslationContext.Provider value={{ t, language, setLanguage, availableLanguages: AVAILABLE_LANGUAGES }}>
      {children}
    </TranslationContext.Provider>
  );
};

export const useTranslation = (): TranslationContextType => {
  const context = useContext(TranslationContext);
  if (!context) {
    throw new Error('useTranslation must be used within TranslationProvider');
  }
  return context;
};
