import type { TreeSession } from "../port/hub";
import { t } from "../i18n";
import { StudioIcon } from "./StudioIcon";
import "../styles/session-quick.css";

export function SessionQuick({ session, label, running, onArchive, onDelete }: {
  session: TreeSession; label: string; running: boolean; onArchive: () => void; onDelete: () => void;
}) {
  return <span className="sessquick">
    <button data-action="session.archive" data-target={session.path} data-value={session.archived ? "restore" : "archive"}
      title={t(session.archived ? "取消归档" : "归档会话")}
      aria-label={t(session.archived ? "取消归档：{title}" : "归档会话：{title}", { title: label })}
      disabled={running} onClick={(ev) => { ev.stopPropagation(); onArchive(); }}><StudioIcon name="archive" /></button>
    <button className="danger" data-action="session.delete" data-target={session.path} title={t("删除会话")}
      aria-label={t("删除会话：{title}", { title: label })}
      onClick={(ev) => { ev.stopPropagation(); onDelete(); }}><StudioIcon name="trash" /></button>
  </span>;
}
