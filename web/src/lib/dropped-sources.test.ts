// 2026-11-06: Test browser-drop traversal and limits with local in-memory files only.
import { describe, expect, it } from "vitest";
import { collectDroppedSources, type DroppedSourceEntry } from "./dropped-sources";

/** Build a file entry without using the host filesystem. */
function fileEntry(name: string, contents = "fixture"): DroppedSourceEntry {
  return { name, isFile: true, isDirectory: false, file: (success) => success(new File([contents], name)) };
}
/** Model paginated directory reads, including the mandatory final empty batch. */
function directory(name: string, batches: DroppedSourceEntry[][]): DroppedSourceEntry {
  return { name, isFile: false, isDirectory: true, createReader: () => {
    let page = 0;
    return { readEntries: (success) => success(batches[page++] || []) };
  } };
}
/** Capture native-like entry handles synchronously, as browsers require during a drop event. */
function transfer(entries: DroppedSourceEntry[]) {
  return { items: entries.map((entry) => ({ kind: "file", webkitGetAsEntry: () => entry, getAsFile: () => null })), files: [] } as unknown as DataTransfer;
}

const excluded = (relative: string) => relative.split("/").some((part) => part === "node_modules" || part === ".env");

describe("collectDroppedSources", () => {
  it("drains every page and preserves same-name files in different directories", async () => {
    const result = await collectDroppedSources(transfer([directory("project", [
      [directory("one", [[fileEntry("index.ts")]])],
      [directory("two", [[fileEntry("index.ts")]])],
    ])]), excluded);
    expect(result.files.map((file) => file.webkitRelativePath)).toEqual(["project/one/index.ts", "project/two/index.ts"]);
  });
  it("does not open excluded directory contents", async () => {
    const blocked: DroppedSourceEntry = { name: "node_modules", isDirectory: true, isFile: false, createReader: () => { throw new Error("must not read"); } };
    const result = await collectDroppedSources(transfer([directory("project", [[blocked, fileEntry("README.md")]])]), excluded);
    expect(result.ignoredDirectories).toEqual(["project/node_modules/"]);
    expect(result.files).toHaveLength(1);
  });
  it("rejects unsafe names and duplicate relative paths", async () => {
    await expect(collectDroppedSources(transfer([fileEntry("../escape")]), excluded)).rejects.toThrow();
    await expect(collectDroppedSources(transfer([fileEntry("same.txt"), fileEntry("same.txt")]), excluded)).rejects.toThrow(/重複/);
  });
  it("enforces file, byte and recursion bounds", async () => {
    const limits = { files: 1, bytes: 2, entries: 10, depth: 1 };
    await expect(collectDroppedSources(transfer([fileEntry("a", "abc")]), excluded, limits)).rejects.toThrow(/上限/);
    await expect(collectDroppedSources(transfer([fileEntry("a", ""), fileEntry("b", "")]), excluded, limits)).rejects.toThrow(/上限/);
    await expect(collectDroppedSources(transfer([directory("a", [[directory("b", [[fileEntry("c", "")]])]])]), excluded, limits)).rejects.toThrow();
  });
  it("supports ordinary dropped files and rejects text-only drops", async () => {
    const result = await collectDroppedSources({ items: [], files: [new File(["x"], "a.txt")] } as unknown as DataTransfer, excluded);
    expect(result.files[0].webkitRelativePath).toBe("a.txt");
    await expect(collectDroppedSources({ items: [], files: [] } as unknown as DataTransfer, excluded)).rejects.toThrow();
  });
  it("reports permission failures instead of silently returning a partial selection", async () => {
    const failed: DroppedSourceEntry = { name: "denied.txt", isFile: true, isDirectory: false, file: (_success, failure) => failure(new DOMException("denied", "NotAllowedError")) };
    await expect(collectDroppedSources(transfer([failed]), excluded)).rejects.toThrow("denied");
  });
});
