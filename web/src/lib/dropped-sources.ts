// 2026-11-06: Read dropped directory entries with bounded traversal, preserving relative paths before upload.
export interface DroppedSourceEntry {
  readonly name: string;
  readonly isFile: boolean;
  readonly isDirectory: boolean;
  file?: (success: (file: File) => void, failure: (error: DOMException) => void) => void;
  createReader?: () => { readEntries: (success: (entries: DroppedSourceEntry[]) => void, failure: (error: DOMException) => void) => void };
}

/** Bound a browser filesystem callback so a cloud/permission failure cannot leave selection busy forever. */
function filesystemResult<Value>(start: (resolve: (value: Value) => void, reject: (error: Error) => void) => void): Promise<Value> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error("ファイルの読み取りが時間切れになりました。ローカルへ保存済みか、読み取り権限があるかを確認してください。")), 20000);
    const finish = (value: Value) => { clearTimeout(timer); resolve(value); };
    const fail = (error: Error) => { clearTimeout(timer); reject(error); };
    try { start(finish, fail); } catch (error) { fail(error instanceof Error ? error : new Error("ファイルを読み取れません。")); }
  });
}

/** Reject ambiguous upload names before combining browser directory entries into server-relative paths. */
function entryPath(parent: string, name: string): string {
  if (!name || name === "." || name === ".." || /[\\/\u0000\r\n:]/.test(name)) throw new Error("不正なファイル名が含まれています。");
  const relative = parent ? `${parent}/${name}` : name;
  if (relative.length > 2048) throw new Error("ファイルのパスが長すぎます。");
  return relative;
}

/** Snapshot drop handles synchronously, then fully drain every directory page without executing file contents. */
export async function collectDroppedSources(
  transfer: Pick<DataTransfer, "items" | "files">,
  excluded: (relative: string) => boolean,
  limits = { files: 2000, bytes: 100 * 1024 * 1024, entries: 20000, depth: 64 },
): Promise<{ files: File[]; ignoredDirectories: string[] }> {
  // DataTransfer is readable only during the drop event; capture entries and files before any await.
  const roots = Array.from(transfer.items || []).filter((item) => item.kind === "file").map((item) => ({
    entry: item.webkitGetAsEntry?.() as unknown as DroppedSourceEntry | null,
    file: item.getAsFile(),
  }));
  const fallback = Array.from(transfer.files || []);
  const files: File[] = [];
  const ignoredDirectories: string[] = [];
  const paths = new Set<string>();
  let visited = 0;
  let includedCount = 0;
  let includedBytes = 0;
  const deadline = Date.now() + 30000;

  /** Register a file without flattening names; reject oversized or colliding selections atomically. */
  function addFile(file: File, relative: string) {
    if (paths.has(relative)) throw new Error(`同じパスが重複しています：${relative}`);
    paths.add(relative);
    if (!excluded(relative)) {
      includedCount += 1; includedBytes += file.size;
      if (includedCount > limits.files || includedBytes > limits.bytes) throw new Error("上限2000ファイル・合計100MiBを超えています。対象フォルダーを小さくしてください。");
    }
    const copy = new File([file], file.name, { type: file.type, lastModified: file.lastModified });
    Object.defineProperty(copy, "webkitRelativePath", { value: relative });
    files.push(copy);
  }

  /** Walk one entry; excluded directories are reported without opening their children. */
  async function visit(entry: DroppedSourceEntry, parent = "", depth = 0): Promise<void> {
    visited += 1;
    if (visited > limits.entries || depth > limits.depth || Date.now() > deadline) throw new Error("フォルダーが大きすぎるか、読み取りに時間がかかっています。小さい範囲に分けてください。");
    const relative = entryPath(parent, entry.name);
    if (entry.isDirectory) {
      if (excluded(relative)) { ignoredDirectories.push(`${relative}/`); return; }
      if (!entry.createReader) throw new Error("このブラウザーではフォルダーを読み取れません。フォルダー選択ボタンをお試しください。");
      const reader = entry.createReader();
      // Chromium returns at most 100 entries per read; keep reading until the empty final batch.
      while (true) {
        if (Date.now() > deadline) throw new Error("フォルダーの読み取りが時間切れになりました。小さい範囲に分けてください。");
        const batch = await filesystemResult<DroppedSourceEntry[]>((resolve, reject) => reader.readEntries(resolve, reject));
        if (!batch.length) break;
        for (const child of batch) await visit(child, relative, depth + 1);
      }
    } else if (entry.isFile && entry.file) {
      const file = await filesystemResult<File>((resolve, reject) => entry.file!(resolve, reject));
      addFile(file, relative);
    } else { throw new Error(`読み取れない項目があります：${relative}`); }
  }

  if (roots.length > limits.entries) throw new Error("ファイル数が多すぎます。");
  if (roots.length) {
    for (const root of roots) {
      if (root.entry) await visit(root.entry);
      else if (root.file) addFile(root.file, entryPath("", root.file.name));
      else throw new Error("項目を読み取れません。Finderからローカルのファイル・フォルダーをドロップしてください。");
    }
  } else {
    if (fallback.length > limits.entries) throw new Error("ファイル数が多すぎます。");
    for (const file of fallback) addFile(file, entryPath("", file.name));
  }
  if (!files.length && !ignoredDirectories.length) throw new Error("ファイルまたはフォルダーをドロップしてください。URLや文章は取り込めません。");
  return { files, ignoredDirectories };
}
