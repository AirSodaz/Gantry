/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_COPILOT_OIDC_ISSUER?: string;
  readonly VITE_COPILOT_OIDC_CLIENT_ID?: string;
  readonly VITE_COPILOT_OIDC_SCOPE?: string;
  readonly VITE_COPILOT_API_BASE?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
