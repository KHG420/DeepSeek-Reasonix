import { createRoot } from "react-dom/client";
import { Appearance } from "../src/ui/Appearance";
import { MockPort } from "../src/port/mock";
import { appearanceCards } from "./appearance-cards.mjs";
import "../src/styles/tokens.css";
import "../src/styles/app.css";
import "../src/styles/studio.css";

const params = new URLSearchParams(location.search);
const root = createRoot(document.getElementById("root")!);
if (params.has("scene")) {
  const theme = params.get("theme")!;
  document.documentElement.dataset.theme = theme;
  const noop = () => {};
  root.render(<div className="prefs"><div className="prefs-page"><Appearance port={new MockPort()} theme={theme} onTheme={noop} contrast="normal" onContrast={noop} weight="normal" onWeight={noop} reloadThemes={noop} look={{}} onLook={noop} /></div></div>);
  const report = () => {
    if (!document.querySelector("#set-font")) return requestAnimationFrame(report);
    parent.postMessage({ appearanceCards: appearanceCards(document) }, location.origin);
  };
  requestAnimationFrame(report);
} else {
  const results = new Map();
  window.addEventListener("message", (e) => {
    if (e.origin !== location.origin || !e.data.appearanceCards) return;
    const r = e.data.appearanceCards;
    results.set(`${r.theme}-${r.width}`, r);
    document.getElementById("summary")!.textContent = [...results.values()].map((r) => `${r.theme} ${r.width}: ${r.pass ? "PASS" : "FAIL"}`).join("; ");
    document.getElementById("result")!.textContent = JSON.stringify([...results.values()], null, 2);
    document.title = results.size === 4 ? ([...results.values()].every((r) => r.pass) ? "Appearance cards PASS" : "Appearance cards FAIL") : "Appearance cards running";
  });
  root.render(<main><h1>Appearance card regression</h1><p id="summary" /><pre id="result" />{["light", "dark"].flatMap((theme) => [1000, 360].map((width) => <iframe key={`${theme}-${width}`} title={`${theme} ${width}`} width={width} height={500} src={`?scene&theme=${theme}`} />))}</main>);
  const style = document.createElement("style");
  style.textContent = "body{overflow:auto;height:auto}main{padding:16px}pre{white-space:pre-wrap;font-size:12px}iframe{display:block;margin:12px 0;border:0}";
  document.head.append(style);
}
