import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { User } from "oidc-client-ts";
import { AuthProvider, useAuth } from "./AuthProvider";

const mocked = vi.hoisted(() => ({
  manager: {
    getUser: vi.fn(),
    signinCallback: vi.fn(),
    signinRedirect: vi.fn(),
    signoutRedirect: vi.fn(),
    events: {
      addUserLoaded: vi.fn(),
      removeUserLoaded: vi.fn(),
      addUserUnloaded: vi.fn(),
      removeUserUnloaded: vi.fn(),
    },
  },
}));

vi.mock("./oidc", () => ({ oidcManager: mocked.manager }));

function Probe() {
  const auth = useAuth();
  return (
    <div>
      <span data-testid="state">
        {auth.isLoading ? "loading" : auth.user ? "signed-in" : "signed-out"}
      </span>
      {auth.error ? <span role="alert">{auth.error.message}</span> : null}
      <button onClick={() => void auth.signIn()}>sign in</button>
      <button onClick={() => void auth.signOut()}>sign out</button>
    </div>
  );
}

const user = {
  profile: { sub: "user-1", preferred_username: "demo" },
  access_token: "token-1",
} as unknown as User;

describe("AuthProvider", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.history.replaceState({}, "", "/");
    mocked.manager.getUser.mockResolvedValue(null);
  });

  it("starts signed out when there is no stored session", async () => {
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent("signed-out"),
    );
    expect(mocked.manager.getUser).toHaveBeenCalledOnce();
  });

  it("handles the authorization callback and removes callback parameters", async () => {
    window.history.replaceState({}, "", "/?code=abc&state=xyz");
    mocked.manager.signinCallback.mockResolvedValue(user);

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );

    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent("signed-in"),
    );
    expect(mocked.manager.signinCallback).toHaveBeenCalledOnce();
    expect(window.location.search).toBe("");
  });

  it("delegates sign in and sign out actions to the OIDC manager", async () => {
    const userEvents = userEvent.setup();
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent("signed-out"),
    );

    await userEvents.click(screen.getByRole("button", { name: "sign in" }));
    await userEvents.click(screen.getByRole("button", { name: "sign out" }));

    expect(mocked.manager.signinRedirect).toHaveBeenCalledOnce();
    expect(mocked.manager.signoutRedirect).toHaveBeenCalledOnce();
  });

  it("surfaces a failed sign-in redirect instead of leaving the button inert", async () => {
    mocked.manager.signinRedirect.mockRejectedValue(
      new Error("Dex is unavailable."),
    );
    const userEvents = userEvent.setup();
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );
    await waitFor(() =>
      expect(screen.getByTestId("state")).toHaveTextContent("signed-out"),
    );

    await userEvents.click(screen.getByRole("button", { name: "sign in" }));

    expect(await screen.findByText("Dex is unavailable.")).toBeInTheDocument();
  });
});
