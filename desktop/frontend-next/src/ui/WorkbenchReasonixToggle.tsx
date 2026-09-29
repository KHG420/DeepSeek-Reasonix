import { t } from "../i18n";
import { StudioIcon } from "./StudioIcon";

export function WorkbenchReasonixToggle({ shown, onToggle }: { shown: boolean; onToggle: () => void }) {
  const label = t(shown ? "隐藏 .reasonix 文件夹" : "显示 .reasonix 文件夹");
  return (
    <button
      className="workbench-open-editor"
      data-action="workbench.reasonix"
      aria-pressed={shown}
      title={label}
      aria-label={label}
      onClick={onToggle}
    >
      <StudioIcon name={shown ? "eye" : "eyeoff"} />
    </button>
  );
}
