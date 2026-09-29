// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "./testkit";
import type { AgentPort, Checkpoint } from "../port/port";
import type { Item } from "../state/session";
import { SayCard } from "./cards/SayCard";
import { useReplyActions } from "./reply";

afterEach(cleanup);

const first = { t: "say", id: "first-reply", text: "first answer", done: true } as Extract<Item, { t: "say" }>;
const second = { t: "say", id: "second-reply", text: "second answer", done: true } as Extract<Item, { t: "say" }>;
const items: Item[] = [
  { t: "user", id: "first-question", text: "first question", msgIndex: 1 },
  first,
  { t: "user", id: "second-question", text: "second question", msgIndex: 3 },
  second,
];
const checkpoints: Checkpoint[] = [
  { turn: 1, msgIndex: 1, prompt: "first question", files: 0 },
  { turn: 2, msgIndex: 3, prompt: "second question", files: 0 },
];

it("regenerates the turn belonging to the clicked older reply", async () => {
  const prepareRewind = vi.fn(async (turn: number) => ({ planId: `plan-${turn}`, canConversation: true }));
  const commitRewind = vi.fn(async () => ({}));
  const submit = vi.fn(async () => true);
  const onError = vi.fn();
  const port = { prepareRewind, commitRewind } as unknown as AgentPort;

  function Replies() {
    const { reply } = useReplyActions({ port, items, checkpoints, running: false, submit, onSettings: vi.fn(), onRunDetail: vi.fn(), onError });
    return <><SayCard item={first} reply={reply} /><SayCard item={second} reply={reply} /></>;
  }
  render(<Replies />);
  await userEvent.click(screen.getAllByRole("button", { name: "重新生成" })[0]);
  await userEvent.click(screen.getByRole("menuitem", { name: /按当前配置重试/ }));

  await waitFor(() => expect(prepareRewind).toHaveBeenCalledWith(1, "conversation"));
  expect(commitRewind).toHaveBeenCalledWith("plan-1");
  expect(submit).toHaveBeenCalledWith("first question");
  expect(onError).not.toHaveBeenCalled();
});

it("does not offer regeneration for a reply whose user turn has no checkpoint", () => {
  function Replies() {
    const { reply } = useReplyActions({
      port: {} as AgentPort, items, checkpoints: checkpoints.slice(1), running: false,
      submit: async () => true, onSettings: vi.fn(), onRunDetail: vi.fn(), onError: vi.fn(),
    });
    return <><SayCard item={first} reply={reply} /><SayCard item={second} reply={reply} /></>;
  }
  render(<Replies />);
  expect(screen.getAllByRole("button", { name: "重新生成" })).toHaveLength(1);
});
