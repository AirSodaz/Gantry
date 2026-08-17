import { createContext, useContext, useMemo, type ReactNode } from "react";
import { CopilotApi } from "./client";
import { useAuth } from "../auth/AuthProvider";

const ApiContext = createContext<CopilotApi | null>(null);

export function ApiProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  const api = useMemo(
    () => new CopilotApi(() => user?.access_token ?? null),
    [user?.access_token],
  );
  return <ApiContext.Provider value={api}>{children}</ApiContext.Provider>;
}

export function useCopilotApi() {
  const api = useContext(ApiContext);
  if (!api) throw new Error("useCopilotApi must be used inside ApiProvider.");
  return api;
}
