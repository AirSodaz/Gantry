import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import type { User } from 'oidc-client-ts';
import { oidcManager } from './oidc';

type AuthContextValue = {
  user: User | null;
  isLoading: boolean;
  error: Error | null;
  signIn: () => Promise<void>;
  signOut: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const callback = window.location.search.includes('code=') && window.location.search.includes('state=');
        const nextUser = callback ? await oidcManager.signinCallback() : await oidcManager.getUser();
        if (callback) window.history.replaceState({}, document.title, window.location.pathname);
        if (active) setUser(nextUser ?? null);
      } catch (cause) {
        if (active) setError(cause instanceof Error ? cause : new Error('Authentication could not be loaded.'));
      } finally {
        if (active) setIsLoading(false);
      }
    };
    void load();
    const onLoaded = (nextUser: User) => setUser(nextUser);
    const onUnloaded = () => setUser(null);
    oidcManager.events.addUserLoaded(onLoaded);
    oidcManager.events.addUserUnloaded(onUnloaded);
    return () => {
      active = false;
      oidcManager.events.removeUserLoaded(onLoaded);
      oidcManager.events.removeUserUnloaded(onUnloaded);
    };
  }, []);

  const value = useMemo<AuthContextValue>(() => ({
    user,
    isLoading,
    error,
    async signIn() {
      setError(null);
      try {
        await oidcManager.signinRedirect();
      } catch (cause) {
        setError(cause instanceof Error ? cause : new Error('Sign in could not be started.'));
      }
    },
    async signOut() {
      try {
        await oidcManager.signoutRedirect();
      } catch (cause) {
        setError(cause instanceof Error ? cause : new Error('Sign out could not be started.'));
      }
    },
  }), [error, isLoading, user]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useAuth must be used inside AuthProvider.');
  return context;
}
