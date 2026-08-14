import { UserManager, WebStorageStateStore } from 'oidc-client-ts';

const issuer = import.meta.env.VITE_ADMIN_OIDC_ISSUER ?? 'http://gantry-dex.localhost:5556/dex';
const clientId = import.meta.env.VITE_ADMIN_OIDC_CLIENT_ID ?? 'gantry-admin-web';
const scope = import.meta.env.VITE_ADMIN_OIDC_SCOPE ?? 'openid profile email audience:server:client_id:gantry-admin-api';

export const oidcManager = new UserManager({
  authority: issuer,
  client_id: clientId,
  redirect_uri: window.location.origin,
  post_logout_redirect_uri: window.location.origin,
  response_type: 'code',
  scope,
  userStore: new WebStorageStateStore({ store: window.sessionStorage }),
  automaticSilentRenew: true,
  monitorSession: false,
});
