/**
 * @vitest-environment jsdom
 */
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient, setApiInstance } from "../api";
import {
  createAuthStore,
  registerAuthStore,
  useAuthStore,
} from "../auth";
import { AuthInitializer } from "./auth-initializer";

const authLogger = vi.hoisted(() => ({
  debug: vi.fn(),
  info: vi.fn(),
  warn: vi.fn(),
  error: vi.fn(),
}));

vi.mock("../logger", async (importOriginal) => {
  const original = await importOriginal<typeof import("../logger")>();
  return { ...original, createLogger: () => authLogger };
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
  useAuthStore.setState({ user: null, isLoading: true });
});

describe("AuthInitializer cookie authentication", () => {
  it("treats an unauthorized response as an anonymous session", async () => {
    // Given
    vi.stubGlobal(
      "fetch",
      vi.fn((input: string | URL | Request) => {
        const url =
          typeof input === "string"
            ? input
            : input instanceof URL
              ? input.href
              : input.url;
        if (url.endsWith("/api/config")) {
          return Promise.resolve(
            new Response("{}", {
              status: 200,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        return Promise.resolve(
          new Response(JSON.stringify({ error: "missing authorization" }), {
            status: 401,
            statusText: "Unauthorized",
            headers: { "Content-Type": "application/json" },
          }),
        );
      }),
    );
    const api = new ApiClient("https://api.example.test");
    setApiInstance(api);
    registerAuthStore(
      createAuthStore({
        api,
        storage: {
          getItem: () => null,
          setItem: vi.fn(),
          removeItem: vi.fn(),
        },
      }),
    );
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const onLogout = vi.fn();

    // When
    render(
      <QueryClientProvider client={queryClient}>
        <AuthInitializer cookieAuth onLogout={onLogout}>
          <div>Anonymous landing page</div>
        </AuthInitializer>
      </QueryClientProvider>,
    );

    // Then
    await waitFor(() => expect(onLogout).toHaveBeenCalledOnce());
    expect(useAuthStore.getState().isLoading).toBe(false);
    expect(authLogger.info).toHaveBeenCalledWith(
      "cookie auth init found no session",
    );
    expect(authLogger.error).not.toHaveBeenCalled();
  });
});
