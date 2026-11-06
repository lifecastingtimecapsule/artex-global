// 2026-11-06: Verify declarative folder selection and drop-to-review behavior without sending files.
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { renderToStaticMarkup } from "react-dom/server";
import { TaskSourceInputs } from "./task-source-inputs";

vi.mock("@/lib/api", () => ({ api: { importGithubSource: vi.fn() } }));
afterEach(cleanup);

/** Supply inert parent callbacks so tests cannot invoke an API or create a task. */
function callbacks() {
  return { busy: false, onBusy: vi.fn(), onPick: vi.fn().mockResolvedValue(undefined), getDraftID: () => "fixture", onAttachments: vi.fn(), onReference: vi.fn() };
}

describe("TaskSourceInputs", () => {
  it("declares directory selection before effects and after remounting", () => {
    const markup = renderToStaticMarkup(<TaskSourceInputs {...callbacks()} />);
    expect(markup).toContain('webkitdirectory=""');
    const first = render(<TaskSourceInputs {...callbacks()} />);
    expect(screen.getByLabelText("フォルダーを追加").hasAttribute("webkitdirectory")).toBe(true);
    first.unmount();
    render(<TaskSourceInputs {...callbacks()} />);
    expect(screen.getByLabelText("フォルダーを追加").hasAttribute("webkitdirectory")).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "GitHub", exact: true }));
    fireEvent.click(screen.getByRole("button", { name: "ファイル・フォルダー" }));
    expect(screen.getByLabelText("フォルダーを追加").hasAttribute("webkitdirectory")).toBe(true);
  });
  it("reviews dropped files before upload and excludes credentials", async () => {
    const props = callbacks();
    render(<TaskSourceInputs {...props} />);
    fireEvent.drop(screen.getByRole("group", { name: "ファイル・フォルダーのドロップ領域" }), {
      dataTransfer: { items: [], files: [new File(["source"], "README.md"), new File(["dummy"], ".env")] },
    });
    await waitFor(() => expect(screen.getByRole("button", { name: "一覧を確認して取り込む" })).toBeTruthy());
    expect(props.onPick).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "一覧を確認して取り込む" }));
    await waitFor(() => expect(props.onPick).toHaveBeenCalledTimes(1));
    const selected = props.onPick.mock.calls[0][0] as File[];
    expect(selected.map((file) => file.name)).toEqual(["README.md"]);
    expect(props.onPick.mock.calls[0][1]).toBe(true);
  });
});
