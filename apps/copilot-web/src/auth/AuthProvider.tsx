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
        const hasCallback = window.location.search.includes('code=') && window.location.search.includes('state=');
        const nextUser = hasCallback
          ? await oidcManager.signinCallback()
          : await oidcManager.getUser();
        if (hasCallback) {
          window.history.replaceState({}, document.title, window.location.pathname);
        }
        if (active) setUser(nextUser ?? null);
      } catch (cause) {
        if (active) setError(cause instanceof Error ? cause : new Error('Authentication could not be loaded.'));
      } finally {
        if (active) setIsLoading(false);
      }
    };
    void load();
    const onUserLoaded = (nextUser: User) => setUser(nextUser);
    const onUserUnloaded = () => setUser(null);
    oidcManager.events.addUserLoaded(onUserLoaded);
    oidcManager.events.addUserUnloaded(onUserUnloaded);
    return () => {
      active = false;
      oidcManager.events.removeUserLoaded(onUserLoaded);
      oidcManager.events.removeUserUnloaded(onUserUnloaded);
    };
  }, []);

  const value = useMemo<AuthContextValue>(() => {
    const signIn = async () => {
      setError(null);
      try {
        await oidcManager.signinRedirect();
      } catch (cause) {
        setError(cause instanceof Error ? cause : new Error('Sign in could not be started.'));
      }
    };
    const signOut = async () => {
      try {
        await oidcManager.signoutRedirect();
      } catch (cause) {
        setError(cause instanceof Error ? cause : new Error('Sign out could not be started.'));
      }
    };
    return { user, isLoading, error, signIn, signOut };
  }, [error, isLoading, user]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useAuth must be used inside AuthProvider.');
  return context;
}
