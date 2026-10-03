// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "./testkit";
import { Market } from "./Market";
import { MockPort } from "../port/mock";
import { boot, STORAGE, t } from "../i18n";
import type { AgentPort } from "../port/port";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  localStorage.setItem(STORAGE, "zh");
  boot();
});

const group = (name: string) => within(screen.getByRole("radiogroup", { name: t(name) }));
const radio = (groupName: string, name: string) => group(groupName).getByRole<HTMLButtonElement>("radio", { name: t(name) });

async function draw(lang = "zh") {
  localStorage.setItem(STORAGE, lang);
  boot();
  const port = new MockPort() as unknown as AgentPort;
  const read = vi.spyOn(port, "marketList");
  render(<Market port={port} onInstalled={() => {}} />);
  await waitFor(() => expect(read).toHaveBeenCalledTimes(1));
  return { port, read };
}

it.each(["zh", "en"])("selects market kinds with all four arrows and preserves their query (%s)", async (lang) => {
  const { read } = await draw(lang);
  await userEvent.tab();
  await userEvent.tab();
  expect(document.activeElement).toBe(radio("类型", "全部"));
  for (const [key, name, kind] of [
    ["ArrowRight", "技能", "skill"], ["ArrowDown", "插件", "plugin"], ["ArrowRight", "MCP 服务", "mcp"],
    ["ArrowDown", "主题", "theme"], ["ArrowRight", "全部", ""], ["ArrowLeft", "主题", "theme"],
    ["ArrowUp", "MCP 服务", "mcp"], ["ArrowLeft", "插件", "plugin"], ["ArrowUp", "技能", "skill"], ["ArrowLeft", "全部", ""],
  ]) {
    await userEvent.keyboard("{" + key + "}");
    expect(document.activeElement).toBe(radio("类型", name));
    expect(radio("类型", name).getAttribute("aria-checked")).toBe("true");
    expect(group("类型").getAllByRole("radio").filter((el) => el.getAttribute("aria-checked") === "true")).toHaveLength(1);
    await waitFor(() => expect(read).toHaveBeenLastCalledWith({ kind, sort: "recommended", q: "", offset: 0, pinned: true }));
  }
});

it.each(["zh", "en"])("selects each market sort with arrows and sends that sort (%s)", async (lang) => {
  const { read } = await draw(lang);
  radio("排序", "推荐").focus();
  for (const [key, name, sort] of [
    ["ArrowRight", "近期热门", "trending"], ["ArrowDown", "安装最多", "installs"], ["ArrowRight", "最新", "new"],
    ["ArrowDown", "推荐", "recommended"], ["ArrowLeft", "最新", "new"], ["ArrowUp", "安装最多", "installs"],
    ["ArrowLeft", "近期热门", "trending"], ["ArrowUp", "推荐", "recommended"],
  ]) {
    await userEvent.keyboard("{" + key + "}");
    expect(document.activeElement).toBe(radio("排序", name));
    expect(radio("排序", name).getAttribute("aria-checked")).toBe("true");
    await waitFor(() => expect(read).toHaveBeenLastCalledWith({ kind: "", sort, q: "", offset: 0, pinned: true }));
  }
});

it.each(["zh", "en"])("gives each filter group one Tab stop after pointer selection (%s)", async (lang) => {
  await draw(lang);
  await userEvent.click(radio("类型", "插件"));
  await userEvent.tab();
  expect(document.activeElement).toBe(radio("排序", "推荐"));
  await userEvent.tab({ shift: true });
  expect(document.activeElement).toBe(radio("类型", "插件"));
  await userEvent.click(radio("排序", "最新"));
  await userEvent.tab();
  expect(document.activeElement).toBe(screen.getByRole("checkbox", { name: t("只看已固定") }));
  await userEvent.tab({ shift: true });
  expect(document.activeElement).toBe(radio("排序", "最新"));
});

it("filters and opens a package using only the keyboard without installing it", async () => {
  const { port, read } = await draw();
  const detail = vi.spyOn(port, "marketDetail");
  const install = vi.spyOn(port, "installMarket");
  await userEvent.tab();
  await userEvent.keyboard("review");
  await waitFor(() => expect(read).toHaveBeenLastCalledWith(expect.objectContaining({ q: "review" })));
  await userEvent.tab();
  await userEvent.keyboard("{ArrowRight}{ArrowRight}");
  await userEvent.tab();
  expect(document.activeElement).toBe(radio("排序", "推荐"));
  await userEvent.keyboard("{ArrowRight}{ArrowRight}{ArrowRight}");
  await userEvent.tab();
  expect(document.activeElement).toBe(screen.getByRole("checkbox", { name: t("只看已固定") }));
  await userEvent.keyboard(" ");
  await waitFor(() => expect(read).toHaveBeenLastCalledWith({ kind: "plugin", sort: "new", q: "review", offset: 0, pinned: false }));
  await screen.findByRole("button", { name: /review-kit/ });
  await userEvent.tab();
  expect(document.activeElement).toBe(screen.getByRole("button", { name: /review-kit/ }));
  await userEvent.keyboard("{Enter}");
  await waitFor(() => expect(detail).toHaveBeenCalledExactlyOnceWith("acme/review-kit"));
  await screen.findByRole("button", { name: t("查看将安装的内容") });
  expect(install).not.toHaveBeenCalled();
});

it("preserves pointer and Space selection", async () => {
  const { read } = await draw();
  await userEvent.click(radio("类型", "主题"));
  radio("排序", "最新").focus();
  await userEvent.keyboard(" ");
  await waitFor(() => expect(read).toHaveBeenLastCalledWith({ kind: "theme", sort: "new", q: "", offset: 0, pinned: true }));
});

it("leaves modified arrows and other keys unchanged", async () => {
  const { read } = await draw();
  for (const [name, option] of [["类型", "全部"], ["排序", "推荐"]]) {
    radio(name, option).focus();
    await userEvent.keyboard("{Control>}{ArrowRight}{/Control}{Alt>}{ArrowLeft}{/Alt}{Meta>}{ArrowDown}{/Meta}x");
    expect(document.activeElement).toBe(radio(name, option));
    expect(radio(name, option).getAttribute("aria-checked")).toBe("true");
  }
  expect(read).toHaveBeenCalledTimes(1);
});
