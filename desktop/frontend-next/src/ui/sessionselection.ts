import { useEffect, useMemo, useState } from "react";
import type { TreeWorkspace } from "../port/hub";

export function useSessionSelection(tree: TreeWorkspace[], folded: Set<string>, whole: Set<string>, shut: boolean, needle: string, scope: string, limit: number) {
  const [selection, setSelection] = useState<Set<string> | null>(null);
  const rows = useMemo(() => new Map(tree.map(ws => [ws.root, whole.has(ws.root) ? ws.sessions : ws.sessions.slice(0, limit)])), [tree, whole, limit]);
  const displayed = useMemo(() => shut ? [] : tree.filter(ws => needle || !folded.has(ws.root)).flatMap(ws => rows.get(ws.root) ?? []), [tree, rows, folded, shut, needle]);
  const selected = [...new Map(displayed.filter(row => selection?.has(row.path)).map(row => [row.path, row])).values()];
  useEffect(() => { setSelection(current => current === null ? null : new Set()); }, [needle, scope]);
  useEffect(() => {
    const paths = new Set(displayed.map(row => row.path));
    setSelection(current => {
      if (!current) return current;
      const next = new Set([...current].filter(path => paths.has(path)));
      return next.size === current.size ? current : next;
    });
  }, [displayed]);
  const toggleSelection = (path: string) => setSelection(current => {
    if (!current) return current;
    const next = new Set(current);
    if (next.has(path)) next.delete(path); else next.add(path);
    return next;
  });
  return { selection, setSelection, toggleSelection, selected, displayed, rows };
}
