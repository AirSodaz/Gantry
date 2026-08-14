import { UserManager, WebStorageStateStore } from 'oidc-client-ts';

const issuer = import.meta.env.VITE_COPILOT_OIDC_ISSUER ?? 'http://gantry-keycloak.localhost:8180/realms/gantry-dev';
const clientId = import.meta.env.VITE_COPILOT_OIDC_CLIENT_ID ?? 'gantry-copilot-web';

export const oidcManager = new UserManager({
  authority: issuer,
  client_id: clientId,
  redirect_uri: window.location.origin,
  post_logout_redirect_uri: window.location.origin,
  response_type: 'code',
  scope: 'openid profile email',
  userStore: new WebStorageStateStore({ store: window.sessionStorage }),
  automaticSilentRenew: true,
  monitorSession: false,
});
