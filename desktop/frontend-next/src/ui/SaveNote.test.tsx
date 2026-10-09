// @vitest-environment jsdom
import { afterEach, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import "./testkit";
import { SaveNote, saveNote } from "./SaveNote";
import { HttpError } from "../port/http_error";

afterEach(cleanup);

it.each([["runtime.saved_while_running", "status", "warn", "已保存，尚未生效"], ["provider.editing_disabled", "alert", "err", "操作未完成"]])("announces %s with its distinct outcome", (code, role, level, title) => {
  render(<SaveNote note={saveNote(new HttpError(409, "diagnostic", { code }))} />);
  const note = screen.getByRole(role);
  expect(note.getAttribute("data-lvl")).toBe(level);
  expect(note.querySelector(".t")?.textContent).toBe(title);
  expect(note.querySelector(".why")?.textContent).not.toBe("diagnostic");
});

it("removes an old notice after a successful retry", () => {
  const view = render(<SaveNote note={{ text: "refused", unapplied: false }} />);
  expect(screen.getByRole("alert")).toBeTruthy();
  view.rerender(<SaveNote note={null} />);
  expect(screen.queryByRole("alert")).toBeNull();
  expect(screen.queryByRole("status")).toBeNull();
});
