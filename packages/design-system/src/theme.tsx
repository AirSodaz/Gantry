import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';

export type ThemeMode = 'light' | 'dark' | 'system';
export type ResolvedTheme = 'light' | 'dark';

export interface ThemeContextValue {
  mode: ThemeMode;
  theme: ResolvedTheme;
  setMode: (mode: ThemeMode) => void;
  toggleTheme: () => void;
}

const STORAGE_KEY = 'gantry_theme_mode';

const ThemeContext = createContext<ThemeContextValue | null>(null);

function getSystemTheme(): ResolvedTheme {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return 'dark';
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function getStoredMode(defaultMode: ThemeMode, key: string): ThemeMode {
  if (typeof window === 'undefined') return defaultMode;
  try {
    const saved = window.localStorage.getItem(key);
    if (saved === 'light' || saved === 'dark' || saved === 'system') {
      return saved;
    }
  } catch {
    // Ignore localStorage access errors in iframe / restricted sandbox
  }
  return defaultMode;
}

export interface ThemeProviderProps {
  children: ReactNode;
  defaultMode?: ThemeMode;
  storageKey?: string;
}

export function ThemeProvider({
  children,
  defaultMode = 'system',
  storageKey = STORAGE_KEY,
}: ThemeProviderProps) {
  const [mode, setModeState] = useState<ThemeMode>(() => getStoredMode(defaultMode, storageKey));
  const [systemTheme, setSystemTheme] = useState<ResolvedTheme>(getSystemTheme);

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    const handler = (e: MediaQueryListEvent) => {
      setSystemTheme(e.matches ? 'dark' : 'light');
    };

    if (mediaQuery.addEventListener) {
      mediaQuery.addEventListener('change', handler);
      return () => mediaQuery.removeEventListener('change', handler);
    } else {
      mediaQuery.addListener(handler);
      return () => mediaQuery.removeListener(handler);
    }
  }, []);

  const resolvedTheme: ResolvedTheme = mode === 'system' ? systemTheme : mode;

  const setMode = useCallback(
    (newMode: ThemeMode) => {
      setModeState(newMode);
      try {
        window.localStorage.setItem(storageKey, newMode);
      } catch {
        // Ignore localStorage error
      }
    },
    [storageKey]
  );

  const toggleTheme = useCallback(() => {
    setModeState((current) => {
      const next = current === 'dark' ? 'light' : 'dark';
      try {
        window.localStorage.setItem(storageKey, next);
      } catch {
        // Ignore localStorage error
      }
      return next;
    });
  }, [storageKey]);

  // Apply data-theme attribute and CSS classes to html/root
  useEffect(() => {
    if (typeof document === 'undefined') return;
    const root = document.documentElement;
    root.setAttribute('data-theme', resolvedTheme);
    root.style.colorScheme = resolvedTheme;
    if (resolvedTheme === 'dark') {
      root.classList.add('dark');
      root.classList.remove('light');
    } else {
      root.classList.add('light');
      root.classList.remove('dark');
    }
  }, [resolvedTheme]);

  const value = useMemo<ThemeContextValue>(
    () => ({
      mode,
      theme: resolvedTheme,
      setMode,
      toggleTheme,
    }),
    [mode, resolvedTheme, setMode, toggleTheme]
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const context = useContext(ThemeContext);
  if (!context) {
    // Fallback if rendered outside ThemeProvider
    return {
      mode: 'dark',
      theme: 'dark',
      setMode: () => undefined,
      toggleTheme: () => undefined,
    };
  }
  return context;
}

export interface ThemeToggleProps {
  className?: string;
  variant?: 'icon' | 'segmented' | 'button';
  size?: 'sm' | 'md';
}

export function ThemeToggle({
  className = '',
  variant = 'icon',
  size = 'md',
}: ThemeToggleProps) {
  const { mode, theme, setMode, toggleTheme } = useTheme();

  if (variant === 'segmented') {
    return (
      <div
        role="radiogroup"
        aria-label="Theme selector"
        className={`ds-theme-segmented ${size === 'sm' ? 'ds-theme-segmented-sm' : ''} ${className}`.trim()}
      >
        <button
          type="button"
          role="radio"
          aria-checked={mode === 'light'}
          title="Light theme"
          onClick={() => setMode('light')}
          className={`ds-theme-seg-btn ${mode === 'light' ? 'ds-theme-seg-active' : ''}`}
        >
          <SunIcon size={size === 'sm' ? 14 : 16} />
          <span>Light</span>
        </button>
        <button
          type="button"
          role="radio"
          aria-checked={mode === 'dark'}
          title="Dark theme"
          onClick={() => setMode('dark')}
          className={`ds-theme-seg-btn ${mode === 'dark' ? 'ds-theme-seg-active' : ''}`}
        >
          <MoonIcon size={size === 'sm' ? 14 : 16} />
          <span>Dark</span>
        </button>
        <button
          type="button"
          role="radio"
          aria-checked={mode === 'system'}
          title="System theme"
          onClick={() => setMode('system')}
          className={`ds-theme-seg-btn ${mode === 'system' ? 'ds-theme-seg-active' : ''}`}
        >
          <LaptopIcon size={size === 'sm' ? 14 : 16} />
          <span>Auto</span>
        </button>
      </div>
    );
  }

  if (variant === 'button') {
    return (
      <button
        type="button"
        onClick={toggleTheme}
        aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
        className={`ds-theme-toggle-btn ${size === 'sm' ? 'ds-theme-toggle-sm' : ''} ${className}`.trim()}
      >
        {theme === 'dark' ? <SunIcon size={16} /> : <MoonIcon size={16} />}
        <span>{theme === 'dark' ? 'Light mode' : 'Dark mode'}</span>
      </button>
    );
  }

  return (
    <button
      type="button"
      onClick={toggleTheme}
      title={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode (current: ${mode})`}
      aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
      className={`ds-icon-button ds-theme-icon-btn ${size === 'sm' ? 'ds-icon-button-sm' : ''} ${className}`.trim()}
    >
      {theme === 'dark' ? (
        <SunIcon size={size === 'sm' ? 16 : 18} />
      ) : (
        <MoonIcon size={size === 'sm' ? 16 : 18} />
      )}
    </button>
  );
}

function SunIcon({ size = 16 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2" />
      <path d="M12 20v2" />
      <path d="m4.93 4.93 1.41 1.41" />
      <path d="m17.66 17.66 1.41 1.41" />
      <path d="M2 12h2" />
      <path d="M20 12h2" />
      <path d="m6.34 17.66-1.41 1.41" />
      <path d="m19.07 4.93-1.41 1.41" />
    </svg>
  );
}

function MoonIcon({ size = 16 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z" />
    </svg>
  );
}

function LaptopIcon({ size = 16 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M20 16V7a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v9m16 0H4m16 0 1.28 2.55a1 1 0 0 1-.9 1.45H3.62a1 1 0 0 1-.9-1.45L4 16" />
    </svg>
  );
}
