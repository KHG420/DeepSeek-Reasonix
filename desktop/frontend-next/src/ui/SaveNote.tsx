import { t } from "../i18n";
import { reason, savedNotApplied } from "../i18n/kernel";

export type SaveOutcome = { text: string; unapplied: boolean };

export function saveNote(error: unknown): SaveOutcome {
  return { text: reason(error), unapplied: savedNotApplied(error) };
}

export function SaveNote({ note }: { note: SaveOutcome | null }) {
  if (!note) return null;
  return (
    <div className="find" data-lvl={note.unapplied ? "warn" : "err"} role={note.unapplied ? "status" : "alert"}>
      <span className="t">{t(note.unapplied ? "已保存，尚未生效" : "操作未完成")}</span>
      <span className="why">{note.text}</span>
    </div>
  );
}
