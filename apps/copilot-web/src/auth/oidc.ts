import { UserManager, WebStorageStateStore } from 'oidc-client-ts';

const issuer = import.meta.env.VITE_COPILOT_OIDC_ISSUER ?? 'http://gantry-dex.localhost:5556/dex';
const clientId = import.meta.env.VITE_COPILOT_OIDC_CLIENT_ID ?? 'gantry-copilot-web';
const scope = import.meta.env.VITE_COPILOT_OIDC_SCOPE ?? 'openid profile email audience:server:client_id:gantry-copilot-api';

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
