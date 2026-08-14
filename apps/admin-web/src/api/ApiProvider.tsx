import { createContext, useContext, useMemo, type ReactNode } from 'react';
import { AdminApi } from './client';
import { useAuth } from '../auth/AuthProvider';

const ApiContext = createContext<AdminApi | null>(null);

export function ApiProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  const api = useMemo(() => new AdminApi(() => user?.access_token ?? null), [user?.access_token]);
  return <ApiContext.Provider value={api}>{children}</ApiContext.Provider>;
}

export function useAdminApi() {
  const api = useContext(ApiContext);
  if (!api) throw new Error('useAdminApi must be used inside ApiProvider.');
  return api;
}
