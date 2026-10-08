import { afterEach, expect, it, vi } from "vitest";
import { SsePort } from "./sse";

afterEach(() => vi.unstubAllGlobals());

it("reads Git identity from the selected runtime's existing endpoint", async () => {
  const info = { repo: true, name: "w", branch: "123abcd", detached: true, added: 2, removed: 1, untracked: 0 };
  const fetch = vi.fn(async (_url: RequestInfo | URL, _init?: RequestInit) => new Response(JSON.stringify(info), { status: 200 }));
  vi.stubGlobal("fetch", fetch);
  expect(await new SsePort("/rt/r2", "r2").workspaceGit()).toEqual(info);
  expect(fetch.mock.calls[0][0]).toBe("/rt/r2/workspace/git");
});
