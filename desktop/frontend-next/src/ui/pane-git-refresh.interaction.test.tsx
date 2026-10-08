// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render, waitFor } from "@testing-library/react";
import "./testkit";
import { Pane } from "./Pane";
import { Composer } from "./Composer";
import { MockPort } from "../port/mock";
import type { WireEvent } from "../port/wire";

Element.prototype.getAnimations ??= () => [];

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

const info = (branch: string, detached = false, repo = true) =>
  ({ repo, name: "w", branch, detached, added: 0, removed: 0, untracked: 0 });

class GitPort extends MockPort {
  workspaceGit = vi.fn(async () => info("trunk"));
  async capabilityScope() { return { ...await super.capabilityScope(), branch: "trunk" }; }
}

function open() {
  const port = new GitPort();
  let emit: (ev: WireEvent) => void = () => {};
  vi.spyOn(port, "subscribe").mockImplementation((on) => {
    emit = on;
    return () => true;
  });
  const view = render(
    <Pane port={port} rt={{ id: "p1", root: "/w", name: "w", base: "/rt/p1" }} title="w" active visible sideHost={null} side={false}
      onFocus={() => {}} onReport={() => {}} onSessionChanged={() => {}} pulse={0} findPulse={0}
      onSettings={() => {}} needsProject={false} onOpenProject={() => {}} onKeepHere={() => {}}
      theme="dark" dockW={560} dockMax={880} onDockW={() => {}} />,
  );
  return { port, ...view, send: (ev: object) => act(() => emit(ev as WireEvent)) };
}

it("reads the session repository instead of the capability scope", async () => {
  const pane = open();
  expect(await pane.findByLabelText("当前 Git 分支：trunk")).toBeTruthy();
  expect(pane.port.workspaceGit).toHaveBeenCalled();
});

it("follows a branch change when a writing tool returns even if the diff count stays the same", async () => {
  const pane = open();
  await pane.findByLabelText("当前 Git 分支：trunk");
  pane.send({ kind: "turn_started" });
  await act(async () => {});
  pane.port.workspaceGit.mockClear();
  pane.send({ kind: "tool_dispatch", tool: { id: "r", name: "read_file", readOnly: true } });
  pane.send({ kind: "tool_result", tool: { id: "r", name: "read_file", readOnly: true, output: "ok" } });
  await act(async () => {});
  expect(pane.port.workspaceGit).not.toHaveBeenCalled();

  pane.port.workspaceGit.mockResolvedValue(info("probe"));
  pane.send({ kind: "tool_dispatch", tool: { id: "w", name: "bash", readOnly: false } });
  pane.send({ kind: "tool_result", tool: { id: "w", name: "bash", readOnly: false, output: "ok" } });
  expect(await pane.findByLabelText("当前 Git 分支：probe")).toBeTruthy();
  expect(pane.getByText("3 个变更")).toBeTruthy();
});

it("refreshes a detached HEAD at the turn boundary with its commit id", async () => {
  const pane = open();
  await pane.findByLabelText("当前 Git 分支：trunk");
  pane.send({ kind: "turn_started" });
  await act(async () => {});
  pane.port.workspaceGit.mockResolvedValue(info("abc1234", true));
  pane.send({ kind: "turn_done" });
  expect(await pane.findByLabelText("当前 Git 分支：分离状态 · abc1234")).toBeTruthy();
});

it("keeps a failed read distinct from a workspace without a repository", async () => {
  const pane = open();
  await pane.findByLabelText("当前 Git 分支：trunk");
  pane.port.workspaceGit.mockResolvedValue(info("", false, false));
  pane.send({ kind: "turn_started" });
  expect(await pane.findByLabelText("不是 Git 仓库")).toBeTruthy();
  expect(pane.queryByText("3 个变更")).toBeNull();
  pane.port.workspaceGit.mockRejectedValue(new Error("offline"));
  pane.send({ kind: "turn_done" });
  expect(await pane.findByLabelText("Git 状态不可用")).toBeTruthy();
});

it("ignores a previous pane's Git response after the connection changes", async () => {
  const old = new GitPort();
  let resolve!: (value: ReturnType<typeof info>) => void;
  old.workspaceGit.mockReturnValue(new Promise((done) => { resolve = done; }));
  const props = { status: null, running: false, onSubmit: async () => true, onChanged: () => {}, onError: () => {} };
  const view = render(<Composer {...props} port={old} />);
  const next = new GitPort();
  next.workspaceGit.mockResolvedValue(info("next"));
  view.rerender(<Composer {...props} port={next} />);
  await view.findByLabelText("当前 Git 分支：next");
  await act(async () => resolve(info("old")));
  await waitFor(() => expect(view.queryByLabelText("当前 Git 分支：old")).toBeNull());
});
