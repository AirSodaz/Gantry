/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_ADMIN_OIDC_ISSUER?: string;
  readonly VITE_ADMIN_OIDC_CLIENT_ID?: string;
  readonly VITE_ADMIN_OIDC_SCOPE?: string;
  readonly VITE_ADMIN_API_BASE?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
